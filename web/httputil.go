package web

import (
	"ministream/constants"
	"ministream/types"
	"ministream/web/apierror"
	"time"

	"github.com/go-playground/validator"
	"github.com/gofiber/fiber/v2"
)

type JSONResultSuccess struct {
	Code    int    `json:"code" example:"200"`
	Message string `json:"message" example:"success"`
}

type JSONResult struct {
	Code    int         `json:"code" example:"200"`
	Message string      `json:"message" example:"success"`
	Data    interface{} `json:"data"`
}

type HTTPError struct {
	Code    int    `json:"code" example:"400"`
	Message string `json:"message" example:"error"`
}

type JSONResultListStreamsProperties struct {
	Code   int                                    `json:"code" example:"200"`
	Result *JSONResultListStreamsPropertiesResult `json:"result"`
}

type JSONResultListStreamsPropertiesResult struct {
	Total int                                         `json:"total" example:"5"`
	Rows  *[]JSONResultListStreamsPropertiesResultRow `json:"rows"`
}

type JSONResultListStreamsPropertiesResultRow struct {
	UUID         types.StreamUUID       `json:"uuid" example:"4ce589e2-b483-467b-8b59-758b339801db"`
	CptMessages  types.Size64           `json:"cptMessages" example:"12345"`
	SizeInBytes  types.Size64           `json:"sizeInBytes" example:"4567890"`
	CreationDate time.Time              `json:"creationDate"`
	LastUpdate   time.Time              `json:"lastUpdate"`
	Properties   types.StreamProperties `json:"properties"`
	LastMsgId    types.MessageId        `json:"lastMsgId"`
}

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
