package rbac

import (
	"errors"
	"fmt"

	"github.com/nbigot/ministream/config"
	"github.com/nbigot/ministream/constants"
	. "github.com/nbigot/ministream/web/apierror"

	"github.com/gofiber/fiber/v2"
)

// Config defines the config for BasicAuth middleware
type RBACHandlerConfig struct {
	// Filter defines a function to skip middleware.
	// Optional. Default: nil
	Filter func(*fiber.Ctx) bool

	// SuccessHandler defines a function which is executed for a valid RBAC authorized action.
	// Optional. Default: nil
	SuccessHandler fiber.Handler

	// ErrorHandler defines a function which is executed for an invalid token.
	// It may be used to define a custom error handler.
	// Optional. Default: 401 RBAC action unauthorized
	ErrorHandler fiber.ErrorHandler

	// Callback handler to get properties to apply JQ on.
	// Optional. Default: nil
	ABACGetPropertiesHandler func(*fiber.Ctx) (interface{}, error)

	RBACContextKey  string
	ABACContextKey  string
	UserContextKey  string
	RolesContextKey string

	ErrorRBACForbidden   int
	ErrorRBACInvalidRule int

	Action string
}

func MakeRBACHandlerConfig(action string, abacGetPropertiesHandler func(*fiber.Ctx) (interface{}, error), successHandler func(c *fiber.Ctx) error, errorHandler func(c *fiber.Ctx, err error) error) *RBACHandlerConfig {
	cfg := RBACHandlerConfig{
		Filter: func(c *fiber.Ctx) bool {
			return !config.Configuration.RBAC.Enable
		},
		SuccessHandler:           successHandler,
		ErrorHandler:             errorHandler,
		ABACGetPropertiesHandler: abacGetPropertiesHandler,
		RBACContextKey:           constants.RBACContextKey,
		ABACContextKey:           constants.ABACContextKey,
		UserContextKey:           constants.UserContextKey,
		RolesContextKey:          constants.RolesContextKey,
		ErrorRBACForbidden:       constants.ErrorRBACForbidden,
		ErrorRBACInvalidRule:     constants.ErrorRBACInvalidRule,
		Action:                   action,
	}

	if cfg.SuccessHandler == nil {
		cfg.SuccessHandler = func(c *fiber.Ctx) error {
			return c.Next()
		}
	}

	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = func(c *fiber.Ctx, err error) error {
			if err != nil {
				httpError := APIError{
					Message:  "rbac error",
					Details:  err.Error(),
					Code:     cfg.ErrorRBACInvalidRule,
					HttpCode: fiber.StatusInternalServerError,
					Err:      err,
				}
				return httpError.HTTPResponse(c)
			} else {
				vErr := ValidationError{FailedField: "action", Tag: "action", Value: cfg.Action}
				httpError := APIError{
					Message:          "rbac action forbidden",
					Code:             cfg.ErrorRBACForbidden,
					HttpCode:         fiber.StatusForbidden,
					ValidationErrors: []*ValidationError{&vErr},
				}
				return httpError.HTTPResponse(c)
			}
		}
	}

	return &cfg
}

func RBACProtected(action string, abacGetPropertiesHandler func(*fiber.Ctx) (interface{}, error), successHandler func(c *fiber.Ctx) error, errorHandler func(c *fiber.Ctx, err error) error) func(*fiber.Ctx) error {
	return NewRBACHandler(MakeRBACHandlerConfig(action, abacGetPropertiesHandler, successHandler, errorHandler))
}

func NewRBACHandler(cfg *RBACHandlerConfig) fiber.Handler {
	// Return middleware handler
	return func(c *fiber.Ctx) error {
		// Filter request to skip middleware
		if cfg.Filter != nil && cfg.Filter(c) {
			return c.Next()
		}

		if c.Locals(constants.SuperUserContextKey) == true {
			return c.Next()
		}

		iRoles := c.Locals(cfg.RolesContextKey)
		if iRoles == nil {
			return cfg.ErrorHandler(c, errors.New("role key not found in locals"))
		}
		roles := iRoles.([]*Role)
		if roles == nil {
			return cfg.ErrorHandler(c, errors.New("roles are empty"))
		}

		user := c.Locals(cfg.UserContextKey).(string)

		// RBAC check for any rule that authorize the requested action
		for _, role := range roles {
			for _, rule := range role.Rules {
				for _, actionName := range rule.Actions {
					if cfg.Action == actionName {
						shouldReturn, returnValue := CheckRBAC(c, cfg, role, rule, &user)
						if shouldReturn {
							return returnValue
						}
					}
				}
			}
		}

		// action not authorized
		c.Locals(cfg.RBACContextKey, map[string]*string{"user": &user, "action": &cfg.Action})
		return cfg.ErrorHandler(c, nil)
	}
}

func CheckRBAC(c *fiber.Ctx, cfg *RBACHandlerConfig, role *Role, rule *Rule, user *string) (bool, error) {
	if rule.Abac != nil {
		c.Locals(cfg.ABACContextKey, rule.Abac)
	}

	if rule.Abac == nil || cfg.ABACGetPropertiesHandler == nil {
		// action authorized
		c.Locals(cfg.RBACContextKey, map[string]*string{"user": user, "role": &role.Id, "rule": &rule.Id, "action": &cfg.Action})
		return true, cfg.SuccessHandler(c)
	}

	properties, err := cfg.ABACGetPropertiesHandler(c)
	if err != nil {
		return true, cfg.ErrorHandler(c, fmt.Errorf("abac properties not found: %s", err.Error()))
	} else {
		if grant, err := CheckABAC(c, properties, rule.Abac); err != nil {
			return true, cfg.ErrorHandler(c, fmt.Errorf("abac properties not found: %s", err))
		} else if grant {
			// action authorized for this specific resource
			c.Locals(cfg.RBACContextKey, map[string]*string{"user": user, "role": &role.Id, "rule": &rule.Id, "action": &cfg.Action, "abac": &rule.Abac.JqDef})
			return true, cfg.SuccessHandler(c)
		}
	}

	return false, nil
}

func CheckABAC(c *fiber.Ctx, properties interface{}, abac *ABAC) (bool, error) {
	if properties == nil {
		return true, nil
	}
	iter := abac.JqFilter.RunWithContext(c.Context(), properties)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, isAnError := v.(error); isAnError {
			return false, err
		}
		if authorized, isBool := v.(bool); isBool {
			return authorized, nil
		} else {
			return false, fmt.Errorf("abac jq filter did not returned a boolean value")
		}
	}

	return true, nil
}
