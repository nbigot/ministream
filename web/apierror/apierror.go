package apierror

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type APIError struct {
	HttpCode         int                `json:"-"`                          // http response status code
	Err              error              `json:"-"`                          // low-level runtime error
	Message          string             `json:"error,omitempty"`            // application-level error message, for debugging
	Details          string             `json:"details,omitempty"`          // application-level error details that best describes the error, for debugging
	Code             int                `json:"code"`                       // application-specific error code
	StreamUUID       uuid.UUID          `json:"streamUUID,omitempty"`       // stream uuid
	ValidationErrors []*ValidationError `json:"validationErrors,omitempty"` // list of errors
}

func (e *APIError) Error() string { return e.Message }

func (e *APIError) HTTPResponse(c *fiber.Ctx) error {
	return c.Status(e.HttpCode).JSON(e)
}

type ValidationError struct {
	FailedField string `json:"failedField,omitempty"`
	Tag         string `json:"tag,omitempty"`
	Value       string `json:"value,omitempty"`
}
