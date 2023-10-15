package rbac

import (
	"fmt"
	"os"

	"github.com/goccy/go-json"
	"go.uber.org/zap"
)

type RBACManager struct {
	Logger     *zap.Logger
	ActionList []string
	Rbac       *RBAC
	enabled    bool
}

var (
	RbacMgr RBACManager
)

func (m *RBACManager) Initialize(logger *zap.Logger, enable bool, actionList []string, filename string) error {
	m.enabled = enable
	m.Logger = logger

	if m.enabled {
		m.ActionList = actionList
		return m.LoadConfiguration(filename)
	} else {
		m.ActionList = make([]string, 0)
		m.Rbac = nil
		return nil
	}
}

func (m *RBACManager) LoadConfiguration(filename string) error {
	m.Logger.Info(
		"Loading RBAC",
		zap.String("topic", "rbac"),
		zap.String("method", "LoadConfiguration"),
		zap.String("filename", filename),
	)

	file, err := os.Open(filename)
	if err != nil {
		m.Logger.Fatal(
			"Can't open RBAC file",
			zap.String("topic", "rbac"),
			zap.String("method", "LoadConfiguration"),
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
			zap.String("topic", "rbac"),
			zap.String("method", "LoadConfiguration"),
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
	if !m.enabled {
		return true
	}

	for _, actionName := range m.ActionList {
		if action == actionName {
			return true
		}
	}
	return false
}

func (m *RBACManager) IsEnabled() bool {
	return m.enabled
}
