package rbac

import (
	"fmt"

	"github.com/itchyny/gojq"
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
