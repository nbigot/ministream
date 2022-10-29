package web

import (
	"ministream/auth"
	"ministream/constants"
	"ministream/web/apierror"

	"github.com/gofiber/fiber/v2"
)

type Pbkdf2Payload struct {
	Digest     string `json:"digest" validate:"required" example:"sha256"`
	Iterations int    `json:"iterations" validate:"min=1,max=10000"`
	Salt       string `json:"salt" validate:"required" example:"thisisarandomsalt"`
	Password   string `json:"password" validate:"required" example:"thisismysecretpassword"`
}

// Ping godoc
// @Summary Ping server
// @Description Ping server
// @ID utils-ping
// @Produce plain
// @Tags Utils
// @Success 200 {string} string "ok"
// @Router /api/v1/utils/ping [get]
func Ping(c *fiber.Ctx) error {
	return c.SendString("ok")
}

// ApiServerUtilsPbkdf2 godoc
// @Summary Generate hash from password
// @Description Generate hash from password
// @ID utils-pbkdf2
// @Accept json
// @Produce json
// @Tags Utils
// @Param payload body Pbkdf2Payload true "Pbkdf2Payload" Format(Pbkdf2Payload)
// @Success 200 {object} web.JSONResultPbkdf2 "successful operation"
// @Failure 400 {object} apierror.APIError
// @Router /api/v1/utils/pbkdf2 [post]
func ApiServerUtilsPbkdf2(c *fiber.Ctx) error {
	var payload Pbkdf2Payload

	if apiErr := GetPayload(c, &payload); apiErr != nil {
		return apiErr.HTTPResponse(c)
	}

	if hash, err := auth.HashPassword(payload.Digest, payload.Iterations, payload.Salt, payload.Password); err == nil {
		return c.JSON(
			JSONResultPbkdf2{
				Code:       200,
				Message:    "success",
				Hash:       hash,
				Digest:     payload.Digest,
				Iterations: payload.Iterations,
				Salt:       payload.Salt,
			},
		)
	} else {
		vErr := apierror.ValidationError{FailedField: "hash", Tag: "parameter", Value: hash}
		httpError := apierror.APIError{
			Message:          "invalid hash value",
			Details:          err.Error(),
			Code:             constants.ErrorInvalidParameterValue,
			HttpCode:         fiber.StatusBadRequest,
			ValidationErrors: []*apierror.ValidationError{&vErr},
			Err:              err,
		}
		return httpError.HTTPResponse(c)
	}
}
