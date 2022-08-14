package web

import (
	"ministream/stream"

	"github.com/gofiber/fiber/v2"
)

// ApiServerStop godoc
// @Summary Stop server
// @Description Stop server
// @ID server-stop
// @Accept json
// @Produce json
// @Tags Admin
// @success 200 {object} web.JSONResultSuccess{} "successful operation"
// @Router /api/v1//admin/server/stop [post]
func ApiServerStop(c *fiber.Ctx) error {
	StopServer()
	return c.JSON(
		JSONResultSuccess{
			Code:    fiber.StatusOK,
			Message: "success",
		},
	)
}

// ApiServerReloadAuth godoc
// @Summary Reload server authentication configuration
// @Description Reload server authentication configuration
// @ID server-reload-auth
// @Accept json
// @Produce json
// @Tags Admin
// @success 200 {object} web.JSONResultSuccess{} "successful operation"
// @Router /api/v1//admin/server/reload/auth [post]
func ApiServerReloadAuth(c *fiber.Ctx) error {
	stream.LoadServerAuthConfig()
	return c.JSON(
		JSONResultSuccess{
			Code:    fiber.StatusOK,
			Message: "success",
		},
	)
}

// ActionJWTRevokeAll godoc
// @Summary Reload server authentication configuration
// @Description Reload server authentication configuration
// @ID server-jwt-revoke-all
// @Accept json
// @Produce json
// @Tags Admin
// @success 200 {object} web.JSONResultSuccess{} "successful operation"
// @Router /api/v1//admin/jwt/revoke [post]
func ActionJWTRevokeAll(c *fiber.Ctx) error {
	JWTRevokeAll()
	return c.JSON(
		JSONResultSuccess{
			Code:    fiber.StatusOK,
			Message: "success",
		},
	)
}