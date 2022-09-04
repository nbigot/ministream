package web

import (
	"ministream/account"
	"ministream/config"
	"ministream/constants"
	"ministream/log"
	"ministream/stream"
	"ministream/web/apierror"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type GetAccountHTTPJsonResult struct {
	ID              uuid.UUID               `json:"id" example:"123489e2-b483-467b-8b59-758b33981234"`
	Name            string                  `json:"name" example:"account name"`
	AccountSettings account.AccountSettings `json:"accountSettings"`
}

// GetAccount godoc
// @Summary Get account
// @Description Get account details
// @ID account-get
// @Accept json
// @Produce json
// @Tags Account
// @success 200 {object} web.JSONResult{data=web.GetAccountHTTPJsonResult{accountSettings=account.AccountSettings}} "successful operation"
// @Router /api/v1/account [get]
func GetAccount(c *fiber.Ctx) error {
	// hide field secretAPIKey
	account := account.GetAccount()
	return c.JSON(
		JSONResult{
			Code:    fiber.StatusOK,
			Message: "success",
			Data:    GetAccountHTTPJsonResult{ID: account.Id, Name: account.Name, AccountSettings: account.AccountSettings},
		},
	)
}

// ValidateApiKey godoc
// @Summary Validate API Key
// @Description Log in a user
// @ID account-validate-api-key
// @Accept json
// @Produce json
// @Tags Account
// @Param API-KEY header string true "API-KEY"
// @Success 200 {object} stream.StreamUUID  "successful operation"
// @Failure 400 {object} apierror.APIError
// @Failure 403 {object} apierror.APIError
// @Router /api/v1/account/validate [get]
func ValidateApiKey(c *fiber.Ctx) error {
	if !config.Configuration.WebServer.JWT.Enable {
		// JWT is not enabled in the configuration
		httpError := apierror.APIError{
			Message:  "bad Request",
			Details:  "JWT is not enabled on server",
			Code:     constants.ErrorJWTNotEnabled,
			HttpCode: fiber.StatusBadRequest,
		}
		return httpError.HTTPResponse(c)
	}

	// The header value for "API-KEY" contains the secret api key
	account := account.GetAccount()
	apiKey := c.Get(constants.APIKEY, "")
	if apiKey == account.SecretAPIKey {
		log.Logger.Info(
			"ValidateApiKey succeeded",
			zap.String("topic", "account"),
			zap.String("method", "ValidateApiKey"),
			zap.String("accountId", account.Id.String()),
			zap.String("ipAddress", c.IP()),
			zap.String("ipAddresses", strings.Join(c.IPs(), ";")),
		)
		return c.JSON(fiber.Map{"status": "success", "valid": true})
	}

	log.Logger.Info(
		"ValidateApiKey failed",
		zap.String("topic", "account"),
		zap.String("method", "ValidateApiKey"),
		zap.String("accountId", account.Id.String()),
		zap.String("ipAddress", c.IP()),
		zap.String("ipAddresses", strings.Join(c.IPs(), ";")),
	)

	// wrong secret value or non existing header key/value
	vErr := apierror.ValidationError{FailedField: constants.APIKEY, Tag: "header"}
	httpError := apierror.APIError{
		Message:          "invalid hash value",
		Details:          "wrong secret value or non existing header key/value",
		Code:             constants.ErrorInvalidParameterValue,
		HttpCode:         fiber.StatusForbidden,
		ValidationErrors: []*apierror.ValidationError{&vErr},
	}
	return httpError.HTTPResponse(c)
}

// LoginAccount godoc
// @Summary Account login
// @Description Account login
// @ID account-login
// @Accept json
// @Produce json
// @Tags Account
// @Param API-KEY header string true "API-KEY"
// @Success 200 {object} stream.LoginAccountResponse "successful operation"
// @Failure 400 {object} apierror.APIError
// @Failure 403 {object} apierror.APIError
// @Router /api/v1/account/login [get]
func LoginAccount(c *fiber.Ctx) error {
	if !config.Configuration.WebServer.JWT.Enable {
		// JWT is not enabled in the configuration
		httpError := apierror.APIError{
			Message:  "bad Request",
			Details:  "JWT is not enabled on server",
			Code:     constants.ErrorJWTNotEnabled,
			HttpCode: fiber.StatusBadRequest,
		}
		return httpError.HTTPResponse(c)
	}

	account := account.GetAccount()
	accountId := account.Id.String()

	// check for super user
	// The header value for "API-KEY" contains the secret api key
	apiKey := c.Get(constants.APIKEY, "")
	success := false
	token := ""
	var err error = nil
	var claims *jwt.MapClaims = nil
	if apiKey == account.SecretAPIKey {
		success, token, claims, _, err = GenerateJWT(accountId, true, "", "")
	}

	if !success || err != nil {
		// wrong secret value or non existing header key/value
		log.Logger.Info(
			"Login failed",
			zap.String("topic", "account"),
			zap.String("method", "LoginAccount"),
			zap.String("accountId", accountId),
			zap.String("ipAddress", c.IP()),
			zap.String("ipAddresses", strings.Join(c.IPs(), ";")),
			zap.String("userAgent", string(c.Context().UserAgent())),
		)
		vErr := apierror.ValidationError{FailedField: constants.APIKEY, Tag: "header"}
		httpError := apierror.APIError{
			Message:          "wrong credentials",
			Code:             constants.ErrorInvalidParameterValue,
			HttpCode:         fiber.StatusForbidden,
			ValidationErrors: []*apierror.ValidationError{&vErr},
		}
		return httpError.HTTPResponse(c)
	}

	log.Logger.Info(
		"Login succeeded",
		zap.String("topic", "account"),
		zap.String("method", "LoginAccount"),
		zap.String("accountId", accountId),
		zap.String("ipAddress", c.IP()),
		zap.String("ipAddresses", strings.Join(c.IPs(), ";")),
		zap.String("userAgent", string(c.Context().UserAgent())),
		zap.String("jti", (*claims)["jti"].(string)),
	)

	response := stream.LoginAccountResponse{
		Status:  "success",
		Message: "login account succeeded",
		JWT:     token,
	}
	return c.JSON(response)
}
