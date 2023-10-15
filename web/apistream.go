package web

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nbigot/ministream/account"
	"github.com/nbigot/ministream/constants"
	"github.com/nbigot/ministream/log"
	"github.com/nbigot/ministream/rbac"
	"github.com/nbigot/ministream/stream"
	"github.com/nbigot/ministream/types"
	"github.com/nbigot/ministream/web/apierror"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/google/uuid"
	"github.com/itchyny/gojq"
	"go.uber.org/zap"
)

// ListStreams godoc
// @Summary List streams
// @Description Get the list of all streams UUIDs
// @ID stream-list
// @Accept json
// @Produce json
// @Tags Stream
// @Param jq query string false "string jq filter" example(".name == \"test 8\"")
// @Success 200 {array} types.StreamUUID "successful operation"
// @Failure 403 {object} apierror.APIError
// @Router /api/v1/streams [get]
func (w *WebAPIServer) ListStreams(c *fiber.Ctx) error {
	var jq *gojq.Query
	var err error

	if jq, err = getJQFromString(c.Query("jq")); err != nil {
		vErr := apierror.ValidationError{FailedField: "jq", Tag: "JQ", Value: c.Query("jq")}
		httpError := apierror.APIError{
			Message:          "invalid jq filter",
			Details:          err.Error(),
			Code:             constants.ErrorInvalidJQFilter,
			HttpCode:         fiber.StatusBadRequest,
			ValidationErrors: []*apierror.ValidationError{&vErr},
			Err:              err,
		}
		return httpError.HTTPResponse(c)
	}

	abacCtx := c.Locals(constants.ABACContextKey)
	var abac *rbac.ABAC = nil
	if abacCtx != nil {
		abac = abacCtx.(*rbac.ABAC)
	}

	var streamsUUIDs types.StreamUUIDList
	var svc = w.service
	if jq == nil && abac == nil {
		streamsUUIDs = svc.GetStreamsUUIDs()
	} else {
		if abac == nil {
			streamsUUIDs = svc.GetStreamsUUIDsFiltered(jq)
		} else {
			streamsUUIDs = svc.GetStreamsUUIDsFiltered(jq, abac.JqFilter)
		}
	}

	return c.JSON(streamsUUIDs)
}

// ListStreamsProperties godoc
// @Summary List streams properties
// @Description Get the streams UUIDs and their properties
// @ID stream-list-and-properties
// @Accept json
// @Produce json
// @Tags Stream
// @Param jq query string false "string jq filter" example(".name == \"test 8\"")
// @Success 200 {object} web.JSONResultListStreamsProperties "successful operation"
// @Failure 403 {object} apierror.APIError
// @Router /api/v1/streams/properties [get]
func (w *WebAPIServer) ListStreamsProperties(c *fiber.Ctx) error {
	if jq, err := getJQFromString(c.Query("jq")); err != nil {
		vErr := apierror.ValidationError{FailedField: "jq", Tag: "JQ", Value: c.Query("jq")}
		httpError := apierror.APIError{
			Message:          "invalid jq filter",
			Details:          err.Error(),
			Code:             constants.ErrorInvalidJQFilter,
			HttpCode:         fiber.StatusBadRequest,
			ValidationErrors: []*apierror.ValidationError{&vErr},
			Err:              err,
		}
		return httpError.HTTPResponse(c)
	} else {
		var rows *[]*stream.Stream
		abacCtx := c.Locals(constants.ABACContextKey)
		var abac *rbac.ABAC = nil
		if abacCtx != nil {
			abac = abacCtx.(*rbac.ABAC)
		}
		if abac == nil {
			rows = w.service.GetStreamsFiltered(jq)
		} else {
			rows = w.service.GetStreamsFiltered(jq, abac.JqFilter)
		}
		res := convertStreamListToJsonResult(rows)
		return c.JSON(res)
	}
}

// CreateStream godoc
// @Summary Create a stream
// @Description Create a new stream
// @ID stream-create
// @Accept json
// @Produce json
// @Tags Stream
// @Success 201 {array} types.StreamInfo
// @Success 400 {object} apierror.APIError
// @Router /api/v1/stream [post]
func (w *WebAPIServer) CreateStream(c *fiber.Ctx) error {
	payload := struct {
		Properties map[string]string `json:"properties" validate:"required,lte=32,dive,keys,gt=0,lte=64,endkeys,max=128,required"`
	}{}

	if apiErr := GetPayload(c, &payload); apiErr != nil {
		return apiErr.HTTPResponse(c)
	}

	s, err := w.service.CreateStream(convertToProperties(payload.Properties))
	if err != nil {
		httpError := apierror.APIError{
			Message:  "cannot create stream",
			Details:  err.Error(),
			Code:     constants.ErrorCantCreateStream,
			HttpCode: fiber.StatusBadRequest,
			Err:      err,
		}
		return httpError.HTTPResponse(c)
	}

	account := account.AccountMgr.GetAccount()
	log.Logger.Info(
		"Stream created",
		zap.String("topic", "stream"),
		zap.String("method", "CreateStream"),
		zap.String("accountId", account.Id.String()),
		zap.String("ipAddress", c.IP()),
		zap.String("ipAddresses", strings.Join(c.IPs(), ";")),
		zap.String("streamUUID", s.GetUUID().String()),
	)

	return c.Status(fiber.StatusCreated).JSON(s.GetInfo())
}

// SetStreamProperties godoc
// @Summary Set stream properties
// @Description Set and replace properties for the given stream
// @ID stream-set-properties
// @Accept json
// @Produce json
// @Tags Stream
// @Param streamuuid path string true "Stream UUID" Format(uuid.UUID)
// @Success 200 {object} types.StreamProperties "successful operation"
// @Success 400 {object} apierror.APIError
// @Router /api/v1/stream/{streamuuid}/properties [post]
func (w *WebAPIServer) SetStreamProperties(c *fiber.Ctx) error {
	payload := struct {
		Properties map[string]string `json:"properties" validate:"required,lte=32,dive,keys,gt=0,lte=64,endkeys,max=256,required"`
	}{}

	if apiErr := GetPayload(c, &payload); apiErr != nil {
		return apiErr.HTTPResponse(c)
	}

	_, streamPtr, apiErr2 := w.GetStreamFromParameter(c)
	if apiErr2 != nil {
		return apiErr2.HTTPResponse(c)
	}

	streamPtr.SetProperties(convertToProperties(payload.Properties))
	return c.JSON(streamPtr.GetProperties())
}

// UpdateStreamProperties godoc
// @Summary Update stream properties
// @Description update properties for the given stream
// @ID stream-update-properties
// @Accept json
// @Produce json
// @Tags Stream
// @Param streamuuid path string true "Stream UUID" Format(uuid.UUID)
// @Success 200 {object} types.StreamProperties "successful operation"
// @Success 400 {object} apierror.APIError
// @Router /api/v1/stream/{streamuuid}/properties [patch]
func (w *WebAPIServer) UpdateStreamProperties(c *fiber.Ctx) error {
	payload := struct {
		Properties map[string]string `json:"properties" validate:"required,lte=32,dive,keys,gt=0,lte=64,endkeys,max=256,required"`
	}{}

	if apiErr := GetPayload(c, &payload); apiErr != nil {
		return apiErr.HTTPResponse(c)
	}

	_, streamPtr, apiErr2 := w.GetStreamFromParameter(c)
	if apiErr2 != nil {
		return apiErr2.HTTPResponse(c)
	}

	streamPtr.UpdateProperties(convertToProperties(payload.Properties))
	return c.JSON(streamPtr.GetProperties())
}

// GetStreamProperties godoc
// @Summary Get stream properties
// @Description Get the properties for the given stream UUID
// @ID stream-get-properties
// @Accept json
// @Produce json
// @Tags Stream
// @Param streamuuid path string true "Stream UUID" Format(uuid.UUID)
// @Success 200 {object} types.StreamProperties "successful operation"
// @Success 400 {object} apierror.APIError
// @Router /api/v1/stream/{streamuuid}/properties [get]
func (w *WebAPIServer) GetStreamProperties(c *fiber.Ctx) error {
	_, streamPtr, apiErr := w.GetStreamFromParameter(c)
	if apiErr != nil {
		return apiErr.HTTPResponse(c)
	}

	return c.JSON(streamPtr.GetProperties())
}

// DeleteStream godoc
// @Summary Delete a stream
// @Description Delete a stream
// @ID stream-delete
// @Accept json
// @Produce json
// @Tags Stream
// @Param streamuuid path string true "Stream UUID" Format(uuid.UUID)
// @success 200 {object} web.JSONResultSuccess{} "successful operation"
// @Success 400 {object} apierror.APIError
// @Router /api/v1/stream/{streamuuid} [delete]
func (w *WebAPIServer) DeleteStream(c *fiber.Ctx) error {
	streamUUID, _, apiErr := w.GetStreamFromParameter(c)
	if apiErr != nil {
		return apiErr.HTTPResponse(c)
	}

	err := w.service.DeleteStream(streamUUID)
	if err != nil {
		httpError := apierror.APIError{
			Message:  "cannot delete stream",
			Details:  err.Error(),
			Code:     constants.ErrorCantDeleteStream,
			HttpCode: fiber.StatusBadRequest,
			Err:      err,
		}
		return httpError.HTTPResponse(c)
	}

	account := account.AccountMgr.GetAccount()
	log.Logger.Info(
		"Stream deleted",
		zap.String("topic", "stream"),
		zap.String("method", "DeleteStream"),
		zap.String("accountId", account.Id.String()),
		zap.String("ipAddress", c.IP()),
		zap.String("ipAddresses", strings.Join(c.IPs(), ";")),
		zap.String("streamUUID", streamUUID.String()),
	)

	return c.JSON(
		JSONResultSuccess{
			Code:    fiber.StatusOK,
			Message: "success",
		},
	)
}

// GetStreamInformation godoc
// @Summary Get stream information
// @Description Get information for the given stream UUID
// @ID stream-get-information
// @Accept json
// @Produce json
// @Tags Stream
// @Param streamuuid path string true "Stream UUID" Format(uuid.UUID)
// @Success 200 {object} types.StreamInfo
// @Success 400 {object} apierror.APIError
// @Router /api/v1/stream/{streamuuid} [get]
func (w *WebAPIServer) GetStreamInformation(c *fiber.Ctx) error {
	_, streamPtr, apiErr := w.GetStreamFromParameter(c)
	if apiErr != nil {
		return apiErr.HTTPResponse(c)
	}

	return c.JSON(streamPtr.GetInfo())
}

// CreateRecordsIterator godoc
// @Summary Create stream records iterator
// @Description Create a record iterator to get records from a given position for the given stream UUID
// @ID stream-create-records-iterator
// @Accept json
// @Produce json
// @Tags Stream
// @Param streamuuid path string true "Stream UUID" Format(uuid.UUID)
// @Success 200 {object} stream.CreateRecordsIteratorResponse
// @Success 400 {object} apierror.APIError
// @Router /api/v1/stream/{streamuuid}/iterator [post]
func (w *WebAPIServer) CreateRecordsIterator(c *fiber.Ctx) error {
	var err error
	var apiError *apierror.APIError
	var streamPtr *stream.Stream
	var streamUUID types.StreamUUID
	var iteratorUUID types.StreamIteratorUUID

	if err = stream.ValidateStreamIteratorRequest(c.Context(), c.Body()); err != nil {
		apiError = &apierror.APIError{
			Message:  "invalid request",
			Details:  err.Error(),
			Code:     constants.ErrorInvalidCreateRecordsIteratorRequest,
			HttpCode: fiber.StatusBadRequest,
			Err:      err,
		}
		return apiError.HTTPResponse(c)
	}

	req := types.StreamIteratorRequest{}
	if apiError = GetPayload(c, &req); apiError != nil {
		return apiError.HTTPResponse(c)
	}

	streamUUID, streamPtr, apiError = w.GetStreamFromParameter(c)
	if apiError != nil {
		return apiError.HTTPResponse(c)
	}

	iteratorUUID, apiError = w.service.CreateRecordsIterator(streamPtr, &req)
	if apiError != nil {
		return apiError.HTTPResponse(c)
	}

	response := stream.CreateRecordsIteratorResponse{
		Status:             "success",
		Message:            "Stream iterator created",
		StreamUUID:         streamUUID,
		StreamIteratorUUID: iteratorUUID,
	}
	return c.JSON(response)
}

// GetRecordsIteratorStats godoc
// @Summary Get statistics about a stream records iterator
// @Description Get statistics for the given stream UUID and stream record iterator UUID
// @ID stream-get-records-iterator-stats
// @Accept json
// @Produce json
// @Tags Stream
// @Param streamuuid path string true "Stream UUID" Format(uuid.UUID)
// @Param streamiteratoruuid path string true "Stream iterator UUID" Format(uuid.UUID)
// @Success 200 {object} stream.GetRecordsIteratorStatsResponse "successful operation"
// @Success 400 {object} apierror.APIError
// @Success 500 {object} apierror.APIError
// @Router /api/v1/stream/{streamuuid}/iterator/{streamiteratoruuid}/stats [get]
func (w *WebAPIServer) GetRecordsIteratorStats(c *fiber.Ctx) error {
	streamUUID, streamPtr, apiErr := w.GetStreamFromParameter(c)
	if apiErr != nil {
		return apiErr.HTTPResponse(c)
	}

	streamIteratorUuid, err := uuid.Parse(c.Params(constants.ParamNameStreamIteratorUuid))
	if err != nil {
		vErr := apierror.ValidationError{
			FailedField: constants.ParamNameStreamIteratorUuid,
			Tag:         "parameter",
			Value:       c.Params(constants.ParamNameStreamIteratorUuid),
		}
		httpError := apierror.APIError{
			Message:          "invalid stream iterator uuid",
			Details:          err.Error(),
			Code:             constants.ErrorInvalidStreamUuid,
			HttpCode:         fiber.StatusBadRequest,
			ValidationErrors: []*apierror.ValidationError{&vErr},
			Err:              err,
		}
		return httpError.HTTPResponse(c)
	}

	var it *stream.StreamIterator
	if it, err = streamPtr.GetIterator(streamIteratorUuid); err != nil {
		httpError := apierror.APIError{
			StreamUUID: streamUUID,
			Message:    "iterator not found",
			Details:    err.Error(),
			Code:       constants.ErrorStreamIteratorNotFound,
			HttpCode:   fiber.StatusBadRequest,
		}
		return httpError.HTTPResponse(c)
	}

	response := stream.GetRecordsIteratorStatsResponse{
		Status:             "success",
		Message:            "",
		StreamUUID:         streamUUID,
		StreamIteratorUUID: streamIteratorUuid,
		LastRecordIdRead:   it.LastRecordIdRead,
		Name:               it.GetName(),
	}
	return c.JSON(response)
}

// CloseRecordsIterator godoc
// @Summary Close a stream records iterator
// @Description Close an existing stream records iterator by it's UUID
// @ID stream-close-records-iterator
// @Accept json
// @Produce json
// @Tags Stream
// @Param streamuuid path string true "Stream UUID" Format(uuid.UUID)
// @Param streamiteratoruuid path string true "Stream iterator UUID" Format(uuid.UUID)
// @Success 200 {object} stream.CloseRecordsIteratorResponse "successful operation"
// @Success 400 {object} apierror.APIError
// @Router /api/v1/stream/{streamuuid}/iterator/{streamiteratoruuid} [delete]
func (w *WebAPIServer) CloseRecordsIterator(c *fiber.Ctx) error {
	streamUUID, streamPtr, apiErr := w.GetStreamFromParameter(c)
	if apiErr != nil {
		return apiErr.HTTPResponse(c)
	}

	streamIteratorUuid, err2 := uuid.Parse(c.Params(constants.ParamNameStreamIteratorUuid))
	if err2 != nil {
		vErr := apierror.ValidationError{
			FailedField: constants.ParamNameStreamIteratorUuid,
			Tag:         "parameter",
			Value:       c.Params(constants.ParamNameStreamIteratorUuid),
		}
		httpError := apierror.APIError{
			StreamUUID:       streamUUID,
			Message:          "invalid stream iterator uuid",
			Details:          err2.Error(),
			Code:             constants.ErrorInvalidStreamUuid,
			HttpCode:         fiber.StatusBadRequest,
			ValidationErrors: []*apierror.ValidationError{&vErr},
			Err:              err2,
		}
		return httpError.HTTPResponse(c)
	}

	if err3 := streamPtr.CloseIterator(streamIteratorUuid); err3 != nil {
		vErr := apierror.ValidationError{
			FailedField: constants.ParamNameStreamIteratorUuid,
			Tag:         "parameter",
			Value:       c.Params(constants.ParamNameStreamIteratorUuid),
		}
		httpError := apierror.APIError{
			StreamUUID:       streamUUID,
			Message:          "cannot close stream iterator uuid",
			Details:          err3.Error(),
			Code:             constants.ErrorCantCloseStreamIterator,
			HttpCode:         fiber.StatusBadRequest,
			ValidationErrors: []*apierror.ValidationError{&vErr},
			Err:              err3,
		}
		return httpError.HTTPResponse(c)
	}

	response := stream.CloseRecordsIteratorResponse{
		Status:             "success",
		Message:            "Stream iterator closed",
		StreamUUID:         streamUUID,
		StreamIteratorUUID: streamIteratorUuid,
	}
	return c.JSON(response)
}

// GetRecords godoc
// @Summary Get stream records
// @Description Get records for the given stream UUID
// @ID stream-get-records
// @Accept json
// @Produce json
// @Tags Stream
// @Param streamuuid path string true "Stream UUID" Format(uuid.UUID)
// @Param streamiteratoruuid path string true "Stream iterator UUID" Format(uuid.UUID)
// @Param maxRecords query int false "int max records" example(10)
// @Success 200 {object} stream.GetStreamRecordsResponse "successful operation"
// @Success 400 {object} apierror.APIError
// @Success 500 {object} apierror.APIError
// @Router /api/v1/stream/{streamuuid}/iterator/{streamiteratoruuid}/records [get]
func (w *WebAPIServer) GetRecords(c *fiber.Ctx) error {
	streamUUID, streamPtr, apiErr := w.GetStreamFromParameter(c)
	if apiErr != nil {
		return apiErr.HTTPResponse(c)
	}

	iteratorUuid, err := uuid.Parse(c.Params(constants.ParamNameStreamIteratorUuid))
	if err != nil {
		vErr := apierror.ValidationError{
			FailedField: constants.ParamNameStreamIteratorUuid,
			Tag:         "parameter",
			Value:       c.Params(constants.ParamNameStreamIteratorUuid),
		}
		httpError := apierror.APIError{
			StreamUUID:       streamUUID,
			Message:          "invalid iterator uuid",
			Details:          err.Error(),
			Code:             constants.ErrorInvalidIteratorUuid,
			HttpCode:         fiber.StatusBadRequest,
			ValidationErrors: []*apierror.ValidationError{&vErr},
			Err:              err,
		}
		return httpError.HTTPResponse(c)
	}

	var maxRecords uint = w.appConfig.Streams.MaxMessagePerGetOperation
	strMaxRecords := c.Query("maxRecords")
	if strMaxRecords != "" {
		var maxRecordsRequested uint64
		maxRecordsRequested, err = strconv.ParseUint(strMaxRecords, 10, 0)
		if err == nil {
			switch v := maxRecordsRequested; {
			case v == 0:
				err = errors.New("value must be positive")
			case v > uint64(maxRecords):
				err = fmt.Errorf("value must cannot exceed limit %d", maxRecords)
			}
		}
		if err != nil {
			vErr := apierror.ValidationError{FailedField: "maxRecords", Tag: "parameter", Value: strMaxRecords}
			httpError := apierror.APIError{
				StreamUUID:       streamUUID,
				Message:          "invalid integer value",
				Details:          err.Error(),
				Code:             constants.ErrorInvalidParameterValue,
				HttpCode:         fiber.StatusBadRequest,
				ValidationErrors: []*apierror.ValidationError{&vErr},
				Err:              err,
			}
			return httpError.HTTPResponse(c)
		}
		maxRecords = uint(maxRecordsRequested)
	}

	response, err2 := streamPtr.GetRecords(c.Context(), iteratorUuid, maxRecords)
	if err2 != nil {
		httpError := apierror.APIError{
			StreamUUID: streamUUID,
			Message:    "cannot get records",
			Details:    err2.Error(),
			Code:       constants.ErrorCantGetMessagesFromStream,
			HttpCode:   fiber.StatusInternalServerError,
			Err:        err2,
		}
		return httpError.HTTPResponse(c)
	}

	return c.JSON(response)
}

// PutRecord godoc
// @Summary Put one record into a stream
// @Description Put a single record into a stream
// @ID stream-put-record
// @Accept json
// @Produce json
// @Tags Stream
// @Param streamuuid path string true "Stream UUID" Format(uuid.UUID)
// @Success 202 {object} stream.PutStreamRecordsResponse "successful operation"
// @Success 400 {object} apierror.APIError
// @Success 500 {object} apierror.APIError
// @Router /api/v1/stream/{streamuuid}/record [put]
func (w *WebAPIServer) PutRecord(c *fiber.Ctx) error {
	startTime := time.Now()
	payload := map[string]interface{}{}

	if err := c.BodyParser(&payload); err != nil {
		httpError := apierror.APIError{
			Message:  "invalid json body format",
			Details:  err.Error(),
			Code:     constants.ErrorCantCreateStream,
			HttpCode: fiber.StatusBadRequest,
			Err:      err,
		}
		return httpError.HTTPResponse(c)
	}

	_, streamPtr, apiErr := w.GetStreamFromParameter(c)
	if apiErr != nil {
		return apiErr.HTTPResponse(c)
	}

	singleMessageId, err2 := streamPtr.PutMessage(c.Context(), payload)
	if err2 != nil {
		httpError := apierror.APIError{
			Message:  "invalid json body format",
			Details:  err2.Error(),
			Code:     constants.ErrorCantPutMessageIntoStream,
			HttpCode: fiber.StatusInternalServerError,
			Err:      err2,
		}
		return httpError.HTTPResponse(c)
	}

	response := stream.PutStreamRecordsResponse{
		Status:     "success",
		StreamUUID: streamPtr.GetUUID(),
		Duration:   time.Since(startTime).Milliseconds(),
		Count:      1,
		MessageIds: []types.MessageId{singleMessageId},
	}
	return c.Status(fiber.StatusAccepted).JSON(response)
}

// PutRecords godoc
// @Summary Put one or multiple records into a stream
// @Description Put one or multiple records into a stream
// @ID stream-put-records
// @Accept json
// @Produce json
// @Tags Stream
// @Param streamuuid path string true "Stream UUID" Format(uuid.UUID)
// @Success 202 {object} stream.PutStreamRecordsResponse "successful operation"
// @Success 400 {object} apierror.APIError
// @Success 500 {object} apierror.APIError
// @Router /api/v1/stream/{streamuuid}/records [put]
func (w *WebAPIServer) PutRecords(c *fiber.Ctx) error {
	var err error
	startTime := time.Now()
	payload := []interface{}{}

	_, streamPtr, apiErr := w.GetStreamFromParameter(c)
	if apiErr != nil {
		return apiErr.HTTPResponse(c)
	}

	// jsonlines (without [])
	ctype := utils.ToLower(utils.UnsafeString(c.Request().Header.ContentType()))
	if ctype == "application/jsonlines" || ctype == "application/x-ndjson" {
		// convert jsonlines into json
		scanner := bufio.NewScanner(bytes.NewReader(c.Body()))
		maxCapacity := 1048576 // 1 MByes
		jsonBuffer := make([]byte, 0, 65536)
		buf := make([]byte, 0, maxCapacity)
		scanner.Buffer(buf, maxCapacity)
		for i := 0; scanner.Scan(); i++ {
			if i == 0 {
				jsonBuffer = append(jsonBuffer, []byte("[")...)
				jsonBuffer = append(jsonBuffer, scanner.Bytes()...)
			} else {
				jsonBuffer = append(jsonBuffer, []byte(",")...)
				jsonBuffer = append(jsonBuffer, scanner.Bytes()...)
			}
		}

		jsonBuffer = append(jsonBuffer, []byte("]")...)
		if err = c.App().Config().JSONDecoder(jsonBuffer, &payload); err != nil {
			httpError := apierror.APIError{
				Message:    "invalid jsonlines body format",
				Details:    err.Error(),
				Code:       constants.ErrorCantDeserializeJsonRecords,
				HttpCode:   fiber.StatusBadRequest,
				StreamUUID: streamPtr.GetUUID(),
				Err:        err,
			}
			return httpError.HTTPResponse(c)
		}
	} else {
		// standard json array (with [])
		if err = c.BodyParser(&payload); err != nil {
			httpError := apierror.APIError{
				Message:    "invalid json body format",
				Details:    err.Error(),
				Code:       constants.ErrorCantDeserializeJsonRecords,
				HttpCode:   fiber.StatusBadRequest,
				StreamUUID: streamPtr.GetUUID(),
				Err:        err,
			}
			return httpError.HTTPResponse(c)
		}
	}

	messageIds, err2 := streamPtr.PutMessages(c.Context(), payload)
	if err2 != nil {
		httpError := apierror.APIError{
			Message:  "invalid json body format",
			Details:  err2.Error(),
			Code:     constants.ErrorCantPutMessagesIntoStream,
			HttpCode: fiber.StatusInternalServerError,
			Err:      err2,
		}
		return httpError.HTTPResponse(c)
	}

	response := stream.PutStreamRecordsResponse{
		Status:     "success",
		StreamUUID: streamPtr.GetUUID(),
		Duration:   time.Since(startTime).Milliseconds(),
		Count:      int64(len(payload)),
		MessageIds: messageIds,
	}
	return c.Status(fiber.StatusAccepted).JSON(response)
}

func (w *WebAPIServer) GetStreamUUIDFromParameter(c *fiber.Ctx) (types.StreamUUID, *apierror.APIError) {
	streamUuid, err := uuid.Parse(c.Params("streamuuid"))
	if err != nil {
		// missing or invalid parameter
		vErr := apierror.ValidationError{FailedField: "streamuuid", Tag: "parameter", Value: c.Params("streamuuid")}
		return streamUuid, &apierror.APIError{
			Message:          "invalid stream uuid",
			Code:             constants.ErrorInvalidStreamUuid,
			HttpCode:         fiber.StatusBadRequest,
			ValidationErrors: []*apierror.ValidationError{&vErr},
			Err:              err,
		}
	}

	return streamUuid, nil
}

func (w *WebAPIServer) GetStreamFromParameter(c *fiber.Ctx) (types.StreamUUID, *stream.Stream, *apierror.APIError) {
	streamUuid, err := w.GetStreamUUIDFromParameter(c)
	if err != nil {
		return streamUuid, nil, err
	}

	streamPtr := w.service.GetStream(streamUuid)
	if streamPtr == nil {
		// stream uuid not found among existing streams
		vErr := apierror.ValidationError{FailedField: "streamuuid", Tag: "parameter", Value: streamUuid.String()}
		return streamUuid, nil, &apierror.APIError{
			Message:          "stream not found",
			Code:             constants.ErrorStreamUuidNotFound,
			HttpCode:         fiber.StatusBadRequest,
			StreamUUID:       streamUuid,
			ValidationErrors: []*apierror.ValidationError{&vErr},
		}
	}

	return streamUuid, streamPtr, nil
}

// RebuildIndex godoc
// @Summary Rebuild the stream index
// @Description Build or rebuild the stream index
// @ID stream-rebuild-index
// @Accept json
// @Produce json
// @Tags Stream
// @Param streamuuid path string true "Stream UUID" Format(uuid.UUID)
// @Success 200 {object} stream.RebuildStreamIndexResponse
// @Success 500 {object} apierror.APIError
// @Router /api/v1/stream/index/{streamuuid}/rebuild [post]
func (w *WebAPIServer) RebuildIndex(c *fiber.Ctx) error {
	startTime := time.Now()

	streamUUID, streamPtr, apiErr := w.GetStreamFromParameter(c)
	if apiErr != nil {
		return apiErr.HTTPResponse(c)
	}

	if itErr := streamPtr.CloseIterators(); itErr != nil {
		httpError := apierror.APIError{
			Message:    "cannot rebuild stream index",
			Details:    itErr.Error(),
			Code:       constants.ErrorCantRebuildStreamIndex,
			HttpCode:   fiber.StatusInternalServerError,
			StreamUUID: streamPtr.GetUUID(),
			Err:        itErr,
		}
		return httpError.HTTPResponse(c)
	}

	indexStats, err := w.service.BuildIndex(streamUUID)
	if err != nil {
		httpError := apierror.APIError{
			Message:    "cannot rebuild stream index",
			Details:    err.Error(),
			Code:       constants.ErrorCantRebuildStreamIndex,
			HttpCode:   fiber.StatusInternalServerError,
			StreamUUID: streamPtr.GetUUID(),
			Err:        err,
		}
		return httpError.HTTPResponse(c)
	}

	account := account.AccountMgr.GetAccount()
	log.Logger.Info(
		"Stream index rebuilt",
		zap.String("topic", "stream"),
		zap.String("method", "RebuildIndex"),
		zap.String("accountId", account.Id.String()),
		zap.String("ipAddress", c.IP()),
		zap.String("ipAddresses", strings.Join(c.IPs(), ";")),
		zap.String("streamUUID", streamUUID.String()),
	)

	response := stream.RebuildStreamIndexResponse{
		Status:     "success",
		Message:    "stream index rebuilt",
		StreamUUID: streamUUID,
		Duration:   time.Since(startTime).Milliseconds(),
		IndexStats: indexStats,
	}
	return c.JSON(response)
}

func convertToProperties(propertiesMap map[string]string) *types.StreamProperties {
	properties := types.StreamProperties{}
	for k, v := range propertiesMap {
		properties[k] = v
	}
	return &properties
}

func getJQFromString(jq string) (*gojq.Query, error) {
	if jq == "" {
		return nil, nil
	}

	jqFilter, err := gojq.Parse(jq)
	if err != nil {
		return nil, err
	} else {
		return jqFilter, nil
	}
}

func convertStreamListToJsonResult(streams *[]*stream.Stream) *JSONResultListStreamsProperties {
	r := make([]JSONResultListStreamsPropertiesResultRow, 0, len(*streams))
	for _, s := range *streams {
		info := s.GetInfo()
		r = append(
			r,
			JSONResultListStreamsPropertiesResultRow{
				UUID:         info.UUID,
				CptMessages:  info.CptMessages,
				SizeInBytes:  info.SizeInBytes,
				CreationDate: info.CreationDate,
				LastUpdate:   info.LastUpdate,
				Properties:   info.Properties,
				LastMsgId:    info.LastMsgId,
			},
		)
	}

	return &JSONResultListStreamsProperties{
		Code: fiber.StatusOK,
		Result: &JSONResultListStreamsPropertiesResult{
			Total: len(*streams),
			Rows:  &r,
		},
	}
}
