package apierror

import (
	"ministream/constants"

	"github.com/go-playground/validator"
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

func ValidateStruct(obj interface{}) []*ValidationError {
	var errors []*ValidationError
	validate := validator.New()
	err := validate.Struct(obj)
	if err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			var element ValidationError
			element.FailedField = err.StructNamespace()
			element.Tag = err.Tag()
			element.Value = err.Param()
			errors = append(errors, &element)
		}
	}
	return errors
}

func GetPayload(c *fiber.Ctx, payload interface{}) *APIError {
	if err := c.BodyParser(&payload); err != nil {
		httpError := APIError{
			Message:  "invalid json body format",
			Details:  err.Error(),
			Code:     constants.ErrorCantDeserializeJson,
			HttpCode: fiber.StatusBadRequest,
			Err:      err,
		}
		return &httpError
	}

	if errors := ValidateStruct(payload); errors != nil {
		httpError := APIError{
			Message:          "invalid json body format",
			Details:          "cannot deserialize json data",
			Code:             constants.ErrorCantDeserializeJson,
			HttpCode:         fiber.StatusBadRequest,
			ValidationErrors: errors,
		}
		return &httpError
	}

	return nil
}
