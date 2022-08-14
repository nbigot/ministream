package rbac

import (
	"fmt"
	"os"

	"github.com/goccy/go-json"
	"github.com/itchyny/gojq"
	"go.uber.org/zap"
)

type RBACManager struct {
	Logger     *zap.Logger
	ActionList []string
	Rbac       *RBAC
}

type User struct {
	Id          string  `json:"id"`
	Description string  `json:"description"`
	Roles       []*Role `json:"roles"`
}

type Role struct {
	Id          string  `json:"id"`
	Description string  `json:"description"`
	Rules       []*Rule `json:"rules"`
}

type Rule struct {
	Id      string   `json:"id"`
	Actions []string `json:"actions"`
	Abac    *ABAC    `json:"abac"`
}

type RBAC struct {
	Users map[string]*User `json:"users"`
	Roles map[string]*Role `json:"roles"`
	Rules map[string]*Rule `json:"rules"`
}

type ABAC struct {
	JqDef    string      `json:"jqdef"`
	JqFilter *gojq.Query `json:"jq"`
}

type RBACSerializeStruct struct {
	Users []struct {
		Id           string   `json:"id"`
		Description  string   `json:"description"`
		SecretAPIKey string   `json:"secretAPIKey"`
		Roles        []string `json:"roles"`
	} `json:"users"`
	Roles []struct {
		Id          string   `json:"id"`
		Description string   `json:"description"`
		Rules       []string `json:"rules"`
	} `json:"roles"`
	Rules []struct {
		Id      string   `json:"id"`
		Actions []string `json:"actions"`
		JQAbac  string   `json:"abac"`
	} `json:"rules"`
}

var (
	RbacMgr RBACManager
)

func (m *RBACManager) Initialize(logger *zap.Logger, actionList []string, filename string) error {
	m.Logger = logger
	m.ActionList = actionList
	return m.LoadRBAC(filename)
}

func (m *RBACManager) LoadRBAC(filename string) error {
	m.Logger.Info(
		"Loading RBAC",
		zap.String("topic", "server"),
		zap.String("method", "LoadRBAC"),
		zap.String("filename", filename),
	)

	file, err := os.Open(filename)
	if err != nil {
		m.Logger.Fatal(
			"Can't open RBAC file",
			zap.String("topic", "server"),
			zap.String("method", "LoadRBAC"),
			zap.String("filename", filename),
			zap.Error(err),
		)
		return err
	}
	defer file.Close()

	srbac := RBACSerializeStruct{}
	jsonDecoder := json.NewDecoder(file)
	err = jsonDecoder.Decode(&srbac)
	if err != nil {
		m.Logger.Fatal(
			"Can't decode json RBAC",
			zap.String("topic", "server"),
			zap.String("method", "LoadRBAC"),
			zap.String("filename", filename),
			zap.Error(err),
		)
		return err
	}

	m.Rbac, err = m.DeserializeRBACConfig(&srbac)
	return err
}

func (m *RBACManager) DeserializeRBACConfig(s *RBACSerializeStruct) (*RBAC, error) {
	rbacObj := RBAC{
		Users: make(map[string]*User),
		Roles: make(map[string]*Role),
		Rules: make(map[string]*Rule),
	}

	for _, sRule := range s.Rules {
		if _, foundRule := rbacObj.Rules[sRule.Id]; foundRule {
			return nil, fmt.Errorf("RBAC Rule id must be unique: %s", sRule.Id)
		}
		for _, action := range sRule.Actions {
			if !m.IsValidAction(action) {
				return nil, fmt.Errorf("RBAC Rule action unknown: %s", action)
			}
		}
		if abac, err := makeABAC(sRule.JQAbac); err != nil {
			return nil, err
		} else {
			rule := Rule{Id: sRule.Id, Actions: sRule.Actions, Abac: abac}
			rbacObj.Rules[sRule.Id] = &rule
		}
	}

	for _, sRole := range s.Roles {
		if _, foundRole := rbacObj.Roles[sRole.Id]; foundRole {
			return nil, fmt.Errorf("RBAC Role id must be unique: %s", sRole.Id)
		}
		role := Role{Id: sRole.Id, Description: sRole.Description, Rules: make([]*Rule, 0)}
		for _, ruleId := range sRole.Rules {
			if pRule, foundRule := rbacObj.Rules[ruleId]; foundRule {
				role.Rules = append(role.Rules, pRule)
			} else {
				return nil, fmt.Errorf("RBAC Rule id %s not found for role: %s", ruleId, sRole.Id)
			}
		}
		rbacObj.Roles[sRole.Id] = &role
	}

	for _, sUser := range s.Users {
		if _, foundUser := rbacObj.Users[sUser.Id]; foundUser {
			return nil, fmt.Errorf("RBAC user id must be unique: %s", sUser.Id)
		}
		user := User{Id: sUser.Id, Description: sUser.Description, Roles: make([]*Role, 0)}
		for _, roleId := range sUser.Roles {
			if pRole, foundRole := rbacObj.Roles[roleId]; foundRole {
				user.Roles = append(user.Roles, pRole)
			} else {
				return nil, fmt.Errorf("RBAC Role id %s not found for user: %s", roleId, sUser.Id)
			}
		}
		rbacObj.Users[sUser.Id] = &user
	}

	return &rbacObj, nil
}

func (m *RBACManager) IsValidAction(action string) bool {
	for _, actionName := range m.ActionList {
		if action == actionName {
			return true
		}
	}
	return false
}

func makeABAC(jqAbac string) (*ABAC, error) {
	if jqAbac == "" {
		return nil, nil
	}

	jq, err := gojq.Parse(jqAbac)
	if err != nil {
		return nil, fmt.Errorf("Can't create ABAC: %s", err.Error())
	}

	abac := ABAC{
		JqDef:    jqAbac,
		JqFilter: jq,
	}

	return &abac, nil
}

func (r *RBAC) GetUser(userId string) (*User, error) {
	if user, foundUser := r.Users[userId]; foundUser {
		return user, nil
	} else {
		return nil, fmt.Errorf("User id not found: %s", userId)
	}
}

func (r *RBAC) GetUserList() []string {
	keys := make([]string, 0, len(r.Users))
	for k := range r.Users {
		keys = append(keys, k)
	}
	return keys
}

func (r *RBAC) GetUserRoles(userId string) ([]string, error) {
	if user, foundUser := r.Users[userId]; foundUser {
		return user.GetRoles(), nil
	} else {
		return nil, fmt.Errorf("User id not found: %s", userId)
	}
}

func (r *RBAC) GetRoles(roleNames *[]string) ([]*Role, error) {
	roles := make([]*Role, 0)
	for _, roleName := range *roleNames {
		if role, foundRole := r.Roles[roleName]; foundRole {
			roles = append(roles, role)
		} else {
			return roles, fmt.Errorf("Unknown role %s", roleName)
		}
	}
	return roles, nil
}

func (u *User) GetRoles() []string {
	roles := make([]string, 0)
	for _, role := range u.Roles {
		roles = append(roles, role.Id)
	}
	return roles
}
