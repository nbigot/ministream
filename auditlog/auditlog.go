package auditlog

import (
	"errors"
	"strings"

	"github.com/nbigot/ministream/constants"
	"github.com/nbigot/ministream/log"
	. "github.com/nbigot/ministream/web/apierror"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"go.uber.org/zap"
)

func RBACHandlerLogAccessGranted(c *fiber.Ctx) error {
	m := c.Locals(constants.RBACContextKey)
	if m == nil {
		m = errors.New("RBAC map attributes not found")
	}

	value := c.Locals(constants.JWTContextKey)
	if value == nil {
		return errors.New("JWT not found")
	}
	token := value.(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)

	log.Logger.Info(
		"RBAC Access granted",
		zap.String("topic", "RBAC"),
		zap.String("method", "RBACHandlerLogAccessGranted"),
		zap.Any("rbac", m),
		zap.String("accountId", claims["account"].(string)),
		zap.String("jti", claims["jti"].(string)),
		zap.String("ipAddress", c.IP()),
		zap.String("ipAddresses", strings.Join(c.IPs(), ";")),
	)
	return c.Next()
}

func RBACHandlerLogAccessDeny(c *fiber.Ctx, err error, auditLogEnabled bool) error {
	action := ""
	internalError := false
	m := c.Locals(constants.RBACContextKey)
	if m == nil {
		internalError = true
	} else {
		reason := m.(map[string]*string)
		action = *reason["action"]
	}

	if auditLogEnabled {
		if internalError {
			log.Logger.Error(
				"RBAC internal error",
				zap.String("topic", "RBAC"),
				zap.String("method", "RBACHandlerLogAccessGranted"),
				zap.Any("rbac", m),
				zap.String("ipAddress", c.IP()),
				zap.String("ipAddresses", strings.Join(c.IPs(), ";")),
				zap.String("reason", "RBAC map attributes not found"),
				zap.String("details", err.Error()),
			)
		} else {
			value := c.Locals(constants.JWTContextKey)
			if value == nil {
				return errors.New("JWT not found")
			}
			token := value.(*jwt.Token)
			claims := token.Claims.(jwt.MapClaims)
			log.Logger.Info(
				"RBAC Access denied",
				zap.String("topic", "RBAC"),
				zap.String("method", "RBACHandlerLogAccessGranted"),
				zap.Any("rbac", m),
				zap.String("accountId", claims["account"].(string)),
				zap.String("jti", claims["jti"].(string)),
				zap.String("ipAddress", c.IP()),
				zap.String("ipAddresses", strings.Join(c.IPs(), ";")),
			)
		}
	}

	if err != nil {
		httpError := APIError{
			Message:  "rbac error",
			Details:  err.Error(),
			Code:     constants.ErrorRBACInvalidRule,
			HttpCode: fiber.StatusInternalServerError,
			Err:      err,
		}
		return httpError.HTTPResponse(c)
	} else {
		httpError := APIError{
			Message:  "rbac action forbidden",
			Details:  action,
			Code:     constants.ErrorRBACForbidden,
			HttpCode: fiber.StatusForbidden,
		}
		return httpError.HTTPResponse(c)
	}
}
