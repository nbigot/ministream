package rbac

import (
	"fmt"

	"go.uber.org/zap"
)

type RBACManager struct {
	// a logger
	Logger *zap.Logger
	// list of all possible actions
	ActionList []string
	// the RBAC configuration
	Rbac *RBAC
	// flag to enable or disable testing the RBAC rules
	enabled bool
}

var (
	RbacMgr *RBACManager
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
	var err error
	if m.Rbac, err = NewRBAC(m.Logger, filename); err != nil {
		return err
	}
	return m.ValidateActions()
}

func (m *RBACManager) ValidateActions() error {
	for _, sRule := range m.Rbac.Rules {
		for _, action := range sRule.Actions {
			if !m.IsValidAction(action) {
				return fmt.Errorf("RBAC Rule action unknown: %s in rule id %s", action, sRule.Id)
			}
		}
	}
	return nil
}

func (m *RBACManager) IsValidAction(action string) bool {
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

func (m *RBACManager) Finalize() {
	m.enabled = false
	m.ActionList = nil
	m.Logger = nil
	m.Rbac = nil
}

func NewRBACManager(logger *zap.Logger, enableRBAC bool, configurationFilenameRBAC string) *RBACManager {
	logger.Info(
		"Loading server RBAC auth configuration",
		zap.String("topic", "rbac"),
		zap.String("method", "NewRBACManager"),
	)

	mgr := &RBACManager{Logger: logger, Rbac: nil, enabled: false}

	if enableRBAC {
		err2 := mgr.Initialize(logger, enableRBAC, ActionList, configurationFilenameRBAC)
		if err2 != nil {
			logger.Fatal("Error while loading RBAC",
				zap.String("topic", "rbac"),
				zap.String("method", "NewRBACManager"),
				zap.String("filename", configurationFilenameRBAC),
				zap.Error(err2),
			)
		}
	} else {
		logger.Info(
			"RBAC is disabled in configuration",
			zap.String("topic", "rbac"),
			zap.String("method", "NewRBACManager"),
		)
	}

	return mgr
}
