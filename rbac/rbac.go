package rbac

import (
	"fmt"
	"os"

	"github.com/goccy/go-json"
	"github.com/itchyny/gojq"
	"go.uber.org/zap"
)

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
		Id          string   `json:"id"`
		Description string   `json:"description"`
		Roles       []string `json:"roles"`
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

func makeABAC(jqAbac string) (*ABAC, error) {
	if jqAbac == "" {
		return nil, nil
	}

	jq, err := gojq.Parse(jqAbac)
	if err != nil {
		return nil, fmt.Errorf("can't create ABAC: %s", err.Error())
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
			return roles, fmt.Errorf("unknown role %s", roleName)
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

func DeserializeRBACConfig(s *RBACSerializeStruct) (*RBAC, error) {
	rbacObj := RBAC{
		Users: make(map[string]*User),
		Roles: make(map[string]*Role),
		Rules: make(map[string]*Rule),
	}

	for _, sRule := range s.Rules {
		if _, foundRule := rbacObj.Rules[sRule.Id]; foundRule {
			return nil, fmt.Errorf("RBAC Rule id must be unique: %s", sRule.Id)
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

func NewRBAC(logger *zap.Logger, filename string) (*RBAC, error) {
	logger.Info(
		"Loading RBAC",
		zap.String("topic", "rbac"),
		zap.String("method", "LoadConfiguration"),
		zap.String("filename", filename),
	)

	file, err := os.Open(filename)
	if err != nil {
		logger.Fatal(
			"Can't open RBAC file",
			zap.String("topic", "rbac"),
			zap.String("method", "LoadConfiguration"),
			zap.String("filename", filename),
			zap.Error(err),
		)
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	srbac := RBACSerializeStruct{}
	jsonDecoder := json.NewDecoder(file)
	err = jsonDecoder.Decode(&srbac)
	if err != nil {
		logger.Fatal(
			"Can't decode json RBAC",
			zap.String("topic", "rbac"),
			zap.String("method", "LoadConfiguration"),
			zap.String("filename", filename),
			zap.Error(err),
		)
		return nil, err
	}

	return DeserializeRBACConfig(&srbac)
}
