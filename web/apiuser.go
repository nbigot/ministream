package web

import (
	"ministream/account"
	"ministream/config"
	"ministream/constants"
	"ministream/log"
	"ministream/rbac"
	"ministream/stream"
	"ministream/web/apierror"
	"strings"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// ListUsers godoc
// @Summary List users
// @Description Get the list of users
// @ID user-list
// @Accept json
// @Produce json
// @Tags User
// @Success 200 {object} stream.StreamUUID "successful operation"
// @Router /api/v1/users [get]
func ListUsers(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "success", "users": rbac.RbacMgr.Rbac.GetUserList()})
}

// LoginUser godoc
// @Summary Logs user into the system
// @Description Logs user into the system
// @ID user-login
// @Accept json
// @Produce json
// @Tags User
// @Param ACCESS-KEY-ID header string true "ACCESS-KEY-ID"
// @Param SECRET-ACCESS-KEY header string true "SECRET-ACCESS-KEY"
// @Success 200 {object} stream.LoginUserResponse "successful operation"
// @Success 400 {object} apierror.APIError
// @Success 403 {object} apierror.APIError
// @Success 500 {object} apierror.APIError
// @Router /api/v1/user/login [get]
func LoginUser(c *fiber.Ctx) error {
	if !config.Configuration.WebServer.JWT.Enable {
		httpError := apierror.APIError{
			Message:  "bad Request",
			Details:  "JWT is not enabled on server",
			Code:     constants.ErrorJWTNotEnabled,
			HttpCode: fiber.StatusBadRequest,
		}
		return httpError.HTTPResponse(c)
	}

	if !config.Configuration.RBAC.Enable {
		httpError := apierror.APIError{
			Message:  "bad Request",
			Details:  "RBAC is not enabled on server",
			Code:     constants.ErrorJWTNotEnabled,
			HttpCode: fiber.StatusBadRequest,
		}
		return httpError.HTTPResponse(c)
	}

	accountId := account.GetAccount().Id.String()
	accessKeyId := c.Get("ACCESS-KEY-ID", "")
	secretAccessKey := c.Get("SECRET-ACCESS-KEY", "")
	success, token, claims, _, err := GenerateJWT(accountId, false, accessKeyId, secretAccessKey)

	if err != nil {
		log.Logger.Error(
			"AuthenticateUser error",
			zap.String("topic", "user"),
			zap.String("method", "LoginUser"),
			zap.String("accountId", accountId),
			zap.String("accessKeyId", accessKeyId),
			zap.String("ipAddress", c.IP()),
			zap.String("ipAddresses", strings.Join(c.IPs(), ";")),
			zap.String("userAgent", string(c.Context().UserAgent())),
			zap.String("jti", (*claims)["jti"].(string)),
			zap.Error(err),
		)
		httpError := apierror.APIError{
			Message:  "authenticate user error",
			Code:     constants.ErrorAuthInternalError,
			HttpCode: fiber.StatusInternalServerError,
		}
		return httpError.HTTPResponse(c)
	}

	if !success {
		// wrong secret value or non existing header key/value
		log.Logger.Info(
			"Login failed",
			zap.String("topic", "user"),
			zap.String("method", "LoginUser"),
			zap.String("accountId", accountId),
			zap.String("accessKeyId", accessKeyId),
			zap.String("ipAddress", c.IP()),
			zap.String("ipAddresses", strings.Join(c.IPs(), ";")),
			zap.String("userAgent", string(c.Context().UserAgent())),
			zap.String("jti", (*claims)["jti"].(string)),
			zap.Error(err),
		)
		httpError := apierror.APIError{
			Message:  "wrong credentials",
			Code:     constants.ErrorWrongCredentials,
			HttpCode: fiber.StatusForbidden,
		}
		return httpError.HTTPResponse(c)
	}

	log.Logger.Info(
		"Login succeeded",
		zap.String("topic", "user"),
		zap.String("method", "LoginUser"),
		zap.String("accountId", accountId),
		zap.String("accessKeyId", accessKeyId),
		zap.String("ipAddress", c.IP()),
		zap.String("ipAddresses", strings.Join(c.IPs(), ";")),
		zap.String("userAgent", string(c.Context().UserAgent())),
		zap.String("jti", (*claims)["jti"].(string)),
	)

	response := stream.LoginUserResponse{
		Status:  "success",
		Message: "login user succeeded",
		JWT:     token,
	}
	return c.JSON(response)
}
