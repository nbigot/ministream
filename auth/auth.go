package auth

import (
	"fmt"

	"github.com/nbigot/ministream/config"
	"go.uber.org/zap"
)

type AuthenticateMethod interface {
	GetMethodName() string
	Initialize(logger *zap.Logger, c *config.AuthConfig) error
	AuthenticateUser(userId string, userPassword string) (bool, error)
}

type AuthManager struct {
	enable     bool
	methods    map[string]AuthenticateMethod
	methodName string
}

var AuthMgr AuthManager

func (m *AuthManager) init() {
	m.enable = true
	m.methodName = ""
	m.methods = map[string]AuthenticateMethod{}
}

func (m *AuthManager) AuthenticateUser(userId string, userPassword string) (bool, error) {
	if m.enable {
		return m.methods[m.methodName].AuthenticateUser(userId, userPassword)
	} else {
		// warning: always grants if disabled!
		return true, nil
	}
}

func (m *AuthManager) Initialize(logger *zap.Logger, c *config.AuthConfig) error {
	m.enable = c.Enable
	m.methodName = c.Method
	if method, foundMethod := m.methods[m.methodName]; foundMethod {
		if err := method.Initialize(logger, c); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("auth method not found: %s", m.methodName)
	}

	return nil
}

func RegisterAuthenticationMethod(m AuthenticateMethod) {
	AuthMgr.methods[m.GetMethodName()] = m
}

func init() {
	AuthMgr.init()
}
