package web

import (
	"github.com/nbigot/ministream/constants"
	"github.com/nbigot/ministream/web/apierror"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

func ValidateStruct(obj interface{}) []*apierror.ValidationError {
	var errors []*apierror.ValidationError
	validate := validator.New()
	err := validate.Struct(obj)
	if err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			var element apierror.ValidationError
			element.FailedField = err.StructNamespace()
			element.Tag = err.Tag()
			element.Value = err.Param()
			errors = append(errors, &element)
		}
	}
	return errors
}

func GetPayload(c *fiber.Ctx, payload interface{}) *apierror.APIError {
	if err := c.BodyParser(&payload); err != nil {
		httpError := apierror.APIError{
			Message:  "invalid json body format",
			Details:  err.Error(),
			Code:     constants.ErrorCantDeserializeJson,
			HttpCode: fiber.StatusBadRequest,
			Err:      err,
		}
		return &httpError
	}

	if errors := ValidateStruct(payload); errors != nil {
		httpError := apierror.APIError{
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
