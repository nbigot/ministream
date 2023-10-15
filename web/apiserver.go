package web

import (
	"github.com/gofiber/fiber/v2"
)

// ApiServerShutdown godoc
// @Summary Shutdown server
// @Description Shutdown server
// @ID server-shutdown
// @Accept json
// @Produce json
// @Tags Admin
// @success 200 {object} web.JSONResultSuccess{} "successful operation"
// @Router /api/v1/admin/server/shutdown [post]
func (w *WebAPIServer) ApiServerShutdown(c *fiber.Ctx) error {
	w.ShutdownServer()
	return c.JSON(
		JSONResultSuccess{
			Code:    fiber.StatusOK,
			Message: "success",
		},
	)
}

// ApiServerRestart godoc
// @Summary Restart server
// @Description Restart server
// @ID server-restart
// @Accept json
// @Produce json
// @Tags Admin
// @success 200 {object} web.JSONResultSuccess{} "successful operation"
// @Router /api/v1/admin/server/restart [post]
func (w *WebAPIServer) ApiServerRestart(c *fiber.Ctx) error {
	w.RestartServer()
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
// @Router /api/v1/admin/jwt/revoke [post]
func (w *WebAPIServer) ActionJWTRevokeAll(c *fiber.Ctx) error {
	JWTRevokeAll()
	return c.JSON(
		JSONResultSuccess{
			Code:    fiber.StatusOK,
			Message: "success",
		},
	)
}
