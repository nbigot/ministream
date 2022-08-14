package web

import (
	"bufio"
	"bytes"
	"ministream/account"
	"ministream/config"
	"ministream/constants"
	"ministream/log"
	"ministream/rbac"
	"ministream/stream"
	. "ministream/web/apierror"
	"strconv"
	"strings"
	"time"

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
// @Success 200 {array} stream.StreamUUID "successful operation"
// @Failure 403 {object} apierror.APIError
// @Router /api/v1/streams [get]
func ListStreams(c *fiber.Ctx) error {
	var streamsUUIDs *[]stream.StreamUUID = nil
	var jq *gojq.Query
	var err error
	if jq, err = getJQFromString(c.Query("jq")); err != nil {
		vErr := ValidationError{FailedField: "jq", Tag: "JQ", Value: c.Query("jq")}
		httpError := APIError{
			Message:          "invalid jq filter",
			Details:          err.Error(),
			Code:             constants.ErrorInvalidJQFilter,
			HttpCode:         fiber.StatusBadRequest,
			ValidationErrors: []*ValidationError{&vErr},
			Err:              err,
		}
		return httpError.HTTPResponse(c)
	}

	abacCtx := c.Locals(constants.ABACContextKey)
	var abac *rbac.ABAC = nil
	if abacCtx != nil {
		abac = abacCtx.(*rbac.ABAC)
	}
	if jq == nil && abac == nil {
		streamsUUIDs = stream.Streams.GetStreamsUUIDs()
	} else {
		if abac == nil {
			streamsUUIDs = stream.Streams.GetStreamsUUIDsFiltered(jq)
		} else {
			streamsUUIDs = stream.Streams.GetStreamsUUIDsFiltered(jq, abac.JqFilter)
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
func ListStreamsProperties(c *fiber.Ctx) error {
	if jq, err := getJQFromString(c.Query("jq")); err != nil {
		vErr := ValidationError{FailedField: "jq", Tag: "JQ", Value: c.Query("jq")}
		httpError := APIError{
			Message:          "invalid jq filter",
			Details:          err.Error(),
			Code:             constants.ErrorInvalidJQFilter,
			HttpCode:         fiber.StatusBadRequest,
			ValidationErrors: []*ValidationError{&vErr},
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
			rows = stream.Streams.GetStreamsFiltered(jq)
		} else {
			rows = stream.Streams.GetStreamsFiltered(jq, abac.JqFilter)
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
// @Success 201 {array} stream.StreamUUID
// @Success 400 {object} apierror.APIError
// @Router /api/v1/stream [post]
func CreateStream(c *fiber.Ctx) error {
	payload := struct {
		Properties map[string]string `json:"properties" validate:"required,lte=32,dive,keys,gt=0,lte=64,endkeys,max=128,required"`
	}{}

	if apiErr := GetPayload(c, &payload); apiErr != nil {
		return apiErr.HTTPResponse(c)
	}

	s, err := stream.Streams.CreateStream(convertToProperties(payload.Properties))
	if err != nil {
		httpError := APIError{
			Message:  "cannot create stream",
			Details:  err.Error(),
			Code:     constants.ErrorCantCreateStream,
			HttpCode: fiber.StatusBadRequest,
			Err:      err,
		}
		return httpError.HTTPResponse(c)
	}

	account := account.GetAccount()
	log.Logger.Info(
		"Stream created",
		zap.String("topic", "stream"),
		zap.String("method", "CreateStream"),
		zap.String("accountId", account.Id.String()),
		zap.String("ipAddress", c.IP()),
		zap.String("ipAddresses", strings.Join(c.IPs(), ";")),
		zap.String("streamUUID", s.UUID.String()),
	)

	return c.Status(fiber.StatusCreated).JSON(s)
}

// SetStreamProperties godoc
// @Summary Set stream properties
// @Description Set and replace properties for the given stream
// @ID stream-set-properties
// @Accept json
// @Produce json
// @Tags Stream
// @Param streamuuid path string true "Stream UUID" Format(uuid.UUID)
// @Success 200 {object} stream.StreamProperties "successful operation"
// @Success 400 {object} apierror.APIError
// @Router /api/v1/stream/{streamuuid}/properties [post]
func SetStreamProperties(c *fiber.Ctx) error {
	payload := struct {
		Properties map[string]string `json:"properties" validate:"required,lte=32,dive,keys,gt=0,lte=64,endkeys,max=256,required"`
	}{}

	if apiErr := GetPayload(c, &payload); apiErr != nil {
		return apiErr.HTTPResponse(c)
	}

	_, streamPtr, apiErr2 := GetStreamFromParameter(c)
	if apiErr2 != nil {
		return apiErr2.HTTPResponse(c)
	}

	streamPtr.SetProperties(convertToProperties(payload.Properties))
	return c.JSON(streamPtr.Properties)
}

// UpdateStreamProperties godoc
// @Summary Update stream properties
// @Description update properties for the given stream
// @ID stream-update-properties
// @Accept json
// @Produce json
// @Tags Stream
// @Param streamuuid path string true "Stream UUID" Format(uuid.UUID)
// @Success 200 {object} stream.StreamProperties "successful operation"
// @Success 400 {object} apierror.APIError
// @Router /api/v1/stream/{streamuuid}/properties [patch]
func UpdateStreamProperties(c *fiber.Ctx) error {
	payload := struct {
		Properties map[string]string `json:"properties" validate:"required,lte=32,dive,keys,gt=0,lte=64,endkeys,max=256,required"`
	}{}

	if apiErr := GetPayload(c, &payload); apiErr != nil {
		return apiErr.HTTPResponse(c)
	}

	_, streamPtr, apiErr2 := GetStreamFromParameter(c)
	if apiErr2 != nil {
		return apiErr2.HTTPResponse(c)
	}

	streamPtr.UpdateProperties(convertToProperties(payload.Properties))
	return c.JSON(streamPtr.Properties)
}

// GetStreamProperties godoc
// @Summary Get stream properties
// @Description Get the properties for the given stream UUID
// @ID stream-get-properties
// @Accept json
// @Produce json
// @Tags Stream
// @Param streamuuid path string true "Stream UUID" Format(uuid.UUID)
// @Success 200 {object} stream.StreamProperties "successful operation"
// @Success 400 {object} apierror.APIError
// @Router /api/v1/stream/{streamuuid}/properties [get]
func GetStreamProperties(c *fiber.Ctx) error {
	_, streamPtr, apiErr := GetStreamFromParameter(c)
	if apiErr != nil {
		return apiErr.HTTPResponse(c)
	}

	return c.JSON(streamPtr.Properties)
}

// GetStreamRawFile godoc
// @Summary Get stream raw file
// @Description Get the raw file for the given stream UUID
// @ID stream-get-raw-file
// @Accept json
// @Produce json
// @Tags Stream
// @Param streamuuid path string true "Stream UUID" Format(uuid.UUID)
// @Success 200 {object} stream.StreamUUID "successful operation"
// @Success 400 {object} apierror.APIError
// @Router /api/v1/stream/{streamuuid}/raw [get]
func GetStreamRawFile(c *fiber.Ctx) error {
	_, streamPtr, apiErr := GetStreamFromParameter(c)
	if apiErr != nil {
		return apiErr.HTTPResponse(c)
	}

	flagCompress := c.AcceptsEncodings("gzip") == "gzip"
	return c.SendFile(streamPtr.GetDataFilePath(), flagCompress)
}

// DeleteStream godoc
// @Summary Delete a stream
// @Description Delete a stream
// @ID stream-delete
// @Accept json
// @Produce json
// @Tags Stream
// @Param streamuuid path string true "Stream UUID" Format(uuid.UUID)
// @Success 200 {object} stream.StreamUUID "successful operation"
// @Success 400 {object} apierror.APIError
// @Router /api/v1/stream/{streamuuid} [delete]
func DeleteStream(c *fiber.Ctx) error {
	streamUUID, _, apiErr := GetStreamFromParameter(c)
	if apiErr != nil {
		return apiErr.HTTPResponse(c)
	}

	err := stream.Streams.DeleteStream(streamUUID)
	if err != nil {
		httpError := APIError{
			Message:  "cannot delete stream",
			Details:  err.Error(),
			Code:     constants.ErrorCantDeleteStream,
			HttpCode: fiber.StatusBadRequest,
			Err:      err,
		}
		return httpError.HTTPResponse(c)
	}

	account := account.GetAccount()
	log.Logger.Info(
		"Stream deleted",
		zap.String("topic", "stream"),
		zap.String("method", "DeleteStream"),
		zap.String("accountId", account.Id.String()),
		zap.String("ipAddress", c.IP()),
		zap.String("ipAddresses", strings.Join(c.IPs(), ";")),
		zap.String("streamUUID", streamUUID.String()),
	)

	return c.JSON(fiber.Map{"status": "success", "message": "Stream deleted", "streamUUID": streamUUID})
}

// GetStreamDescription godoc
// @Summary Get stream description
// @Description Get the description for the given stream UUID
// @ID stream-get-description
// @Accept json
// @Produce json
// @Tags Stream
// @Param streamuuid path string true "Stream UUID" Format(uuid.UUID)
// @Success 200 {object} stream.Stream "successful operation"
// @Success 400 {object} apierror.APIError
// @Router /api/v1/stream/{streamuuid} [get]
func GetStreamDescription(c *fiber.Ctx) error {
	_, streamPtr, apiErr := GetStreamFromParameter(c)
	if apiErr != nil {
		return apiErr.HTTPResponse(c)
	}

	return c.JSON(streamPtr)
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
func CreateRecordsIterator(c *fiber.Ctx) error {
	var err error

	if err = stream.ValidateStreamIteratorRequest(c.Context(), c.Body()); err != nil {
		httpError := APIError{
			Message:  "invalid request",
			Details:  err.Error(),
			Code:     constants.ErrorInvalidCreateRecordsIteratorRequest,
			HttpCode: fiber.StatusBadRequest,
			Err:      err,
		}
		return httpError.HTTPResponse(c)
	}

	req := stream.StreamIteratorRequest{}
	if httpError := GetPayload(c, &req); httpError != nil {
		return httpError.HTTPResponse(c)
	}

	streamUUID, streamPtr, httpError2 := GetStreamFromParameter(c)
	if httpError2 != nil {
		return httpError2.HTTPResponse(c)
	}

	var iter *stream.StreamIterator
	if iter, err = stream.CreateRecordsIterator(&req); err != nil {
		httpError := APIError{
			Message:    "cannot create stream iterator",
			Details:    err.Error(),
			Code:       constants.ErrorInvalidCreateRecordsIteratorRequest,
			HttpCode:   fiber.StatusBadRequest,
			StreamUUID: streamUUID,
			Err:        err,
		}
		return httpError.HTTPResponse(c)
	}

	if err = streamPtr.AddIterator(iter); err != nil {
		httpError := APIError{
			Message:    "cannot create stream iterator",
			Details:    err.Error(),
			Code:       constants.ErrorInvalidCreateRecordsIteratorRequest,
			HttpCode:   fiber.StatusBadRequest,
			StreamUUID: streamUUID,
			Err:        err,
		}
		return httpError.HTTPResponse(c)
	}

	response := stream.CreateRecordsIteratorResponse{
		Status:             "success",
		Message:            "Stream iterator created",
		StreamUUID:         streamUUID,
		StreamIteratorUUID: iter.UUID,
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
// @Param iteratoruuid path string true "Iterator UUID" Format(uuid.UUID)
// @Success 200 {object} stream.StreamUUID "successful operation"
// @Success 400 {object} apierror.APIError
// @Success 500 {object} apierror.APIError
// @Router /api/v1/stream/{streamuuid}/iterator/{iteratoruuid}/stats [get]
func GetRecordsIteratorStats(c *fiber.Ctx) error {
	streamUUID, streamPtr, apiErr := GetStreamFromParameter(c)
	if apiErr != nil {
		return apiErr.HTTPResponse(c)
	}

	streamIteratorUuid, err := uuid.Parse(c.Params("streamiteratoruuid"))
	if err != nil {
		vErr := ValidationError{FailedField: "streamiteratoruuid", Tag: "parameter", Value: c.Params("streamiteratoruuid")}
		httpError := APIError{
			Message:          "invalid stream iterator uuid",
			Details:          err.Error(),
			Code:             constants.ErrorInvalidStreamUuid,
			HttpCode:         fiber.StatusBadRequest,
			ValidationErrors: []*ValidationError{&vErr},
			Err:              err,
		}
		return httpError.HTTPResponse(c)
	}

	var it *stream.StreamIterator
	if it, err = streamPtr.GetIterator(streamIteratorUuid); err != nil {
		httpError := APIError{
			Message:  "iterator not found",
			Details:  err.Error(),
			Code:     constants.ErrorStreamIteratorNotFound,
			HttpCode: fiber.StatusBadRequest,
		}
		return httpError.HTTPResponse(c)
	}

	response := stream.GetRecordsIteratorStatsResponse{
		Status:             "success",
		Message:            "",
		StreamUUID:         streamUUID,
		StreamIteratorUUID: streamIteratorUuid,
		LastMessageIdRead:  it.LastMessageIdRead,
		FileOffset:         it.FileOffset,
	}
	return c.JSON(response)

	// if err3 := streamPtr.CloseIterator(streamIteratorUuid); err3 != nil {
	// 	vErr := ValidationError{FailedField: "streamiteratoruuid", Tag: "parameter", Value: c.Params("streamiteratoruuid")}
	// 	httpError := APIError{
	// 		Message:          "cannot close stream iterator uuid",
	// 		Details:          err3.Error(),
	// 		Code:             constants.ErrorCantCloseStreamIterator,
	// 		HttpCode:         fiber.StatusBadRequest,
	// 		ValidationErrors: []*ValidationError{&vErr},
	// 		Err:              err3,
	// 	}
	// 	return httpError.HTTPResponse(c)
	// }

	// response := stream.CloseRecordsIteratorResponse{
	// 	Status:             "success",
	// 	Message:            "Stream iterator closed",
	// 	StreamUUID:         streamUuid,
	// 	StreamIteratorUUID: streamIteratorUuid,
	// }
	// return c.JSON(response)
}

// CloseRecordsIterator godoc
// @Summary Close a stream records iterator
// @Description Close an existing stream records iterator by it's UUID
// @ID stream-close-records-iterator
// @Accept json
// @Produce json
// @Tags Stream
// @Param streamuuid path string true "Stream UUID" Format(uuid.UUID)
// @Success 200 {object} stream.CloseRecordsIteratorResponse "successful operation"
// @Success 400 {object} apierror.APIError
// @Router /api/v1/stream/{streamuuid}/iterator [delete]
func CloseRecordsIterator(c *fiber.Ctx) error {
	streamUuid, streamPtr, apiErr := GetStreamFromParameter(c)
	if apiErr != nil {
		return apiErr.HTTPResponse(c)
	}

	paramNameIteratoruuid := "iteratoruuid"
	streamIteratorUuid, err2 := uuid.Parse(c.Params(paramNameIteratoruuid))
	if err2 != nil {
		vErr := ValidationError{FailedField: paramNameIteratoruuid, Tag: "parameter", Value: c.Params(paramNameIteratoruuid)}
		httpError := APIError{
			Message:          "invalid stream iterator uuid",
			Details:          err2.Error(),
			Code:             constants.ErrorInvalidStreamUuid,
			HttpCode:         fiber.StatusBadRequest,
			ValidationErrors: []*ValidationError{&vErr},
			Err:              err2,
		}
		return httpError.HTTPResponse(c)
	}

	if err3 := streamPtr.CloseIterator(streamIteratorUuid); err3 != nil {
		vErr := ValidationError{FailedField: paramNameIteratoruuid, Tag: "parameter", Value: c.Params(paramNameIteratoruuid)}
		httpError := APIError{
			Message:          "cannot close stream iterator uuid",
			Details:          err3.Error(),
			Code:             constants.ErrorCantCloseStreamIterator,
			HttpCode:         fiber.StatusBadRequest,
			ValidationErrors: []*ValidationError{&vErr},
			Err:              err3,
		}
		return httpError.HTTPResponse(c)
	}

	response := stream.CloseRecordsIteratorResponse{
		Status:             "success",
		Message:            "Stream iterator closed",
		StreamUUID:         streamUuid,
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
// @Param iteratoruuid path string true "Iterator UUID" Format(uuid.UUID)
// @Param maxRecords query int false "int max records" example(10)
// @Success 200 {object} stream.GetStreamRecordsResponse "successful operation"
// @Success 400 {object} apierror.APIError
// @Success 500 {object} apierror.APIError
// @Router /api/v1/stream/{streamuuid}/iterator/{iteratoruuid}/records [get]
func GetRecords(c *fiber.Ctx) error {
	_, streamPtr, apiErr := GetStreamFromParameter(c)
	if apiErr != nil {
		return apiErr.HTTPResponse(c)
	}

	iteratorUuid, err := uuid.Parse(c.Params("iteratoruuid"))
	if err != nil {
		vErr := ValidationError{FailedField: "iteratoruuid", Tag: "parameter", Value: c.Params("iteratoruuid")}
		httpError := APIError{
			Message:          "invalid iterator uuid",
			Details:          err.Error(),
			Code:             constants.ErrorInvalidIteratorUuid,
			HttpCode:         fiber.StatusBadRequest,
			ValidationErrors: []*ValidationError{&vErr},
			Err:              err,
		}
		return httpError.HTTPResponse(c)
	}

	var maxRecords int = config.Configuration.Streams.MaxMessagePerGetOperation
	strMaxRecords := c.Query("maxRecords")
	if strMaxRecords != "" {
		maxRecords, err = strconv.Atoi(strMaxRecords)
		if err != nil {
			vErr := ValidationError{FailedField: "maxRecords", Tag: "parameter", Value: strMaxRecords}
			httpError := APIError{
				Message:          "invalid integer value",
				Details:          err.Error(),
				Code:             constants.ErrorInvalidParameterValue,
				HttpCode:         fiber.StatusBadRequest,
				ValidationErrors: []*ValidationError{&vErr},
				Err:              err,
			}
			return httpError.HTTPResponse(c)
		}
		if maxRecords > config.Configuration.Streams.MaxMessagePerGetOperation {
			maxRecords = config.Configuration.Streams.MaxMessagePerGetOperation
		}
	}

	response, err2 := streamPtr.GetMessages(c.Context(), iteratorUuid, maxRecords)
	if err2 != nil {
		httpError := APIError{
			Message:  "cannot get records",
			Details:  err2.Error(),
			Code:     constants.ErrorCantGetMessagesFromStream,
			HttpCode: fiber.StatusInternalServerError,
			Err:      err2,
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
func PutRecord(c *fiber.Ctx) error {
	startTime := time.Now()
	payload := map[string]interface{}{}

	if err := c.BodyParser(&payload); err != nil {
		httpError := APIError{
			Message:  "invalid json body format",
			Details:  err.Error(),
			Code:     constants.ErrorCantCreateStream,
			HttpCode: fiber.StatusBadRequest,
			Err:      err,
		}
		return httpError.HTTPResponse(c)
	}

	_, streamPtr, apiErr := GetStreamFromParameter(c)
	if apiErr != nil {
		return apiErr.HTTPResponse(c)
	}

	singleMessageId, err2 := streamPtr.PutMessage(c.Context(), payload)
	if err2 != nil {
		httpError := APIError{
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
		StreamUUID: streamPtr.UUID,
		Duration:   time.Since(startTime).Milliseconds(),
		Count:      1,
		MessageIds: []stream.MessageId{singleMessageId},
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
func PutRecords(c *fiber.Ctx) error {
	var err error
	startTime := time.Now()
	payload := []interface{}{}

	_, streamPtr, apiErr := GetStreamFromParameter(c)
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
			httpError := APIError{
				Message:    "invalid jsonlines body format",
				Details:    err.Error(),
				Code:       constants.ErrorCantDeserializeJsonRecords,
				HttpCode:   fiber.StatusBadRequest,
				StreamUUID: streamPtr.UUID,
				Err:        err,
			}
			return httpError.HTTPResponse(c)
		}
	} else {
		// standard json array (with [])
		if err = c.BodyParser(&payload); err != nil {
			httpError := APIError{
				Message:    "invalid json body format",
				Details:    err.Error(),
				Code:       constants.ErrorCantDeserializeJsonRecords,
				HttpCode:   fiber.StatusBadRequest,
				StreamUUID: streamPtr.UUID,
				Err:        err,
			}
			return httpError.HTTPResponse(c)
		}
	}

	messageIds, err2 := streamPtr.PutMessages(c.Context(), payload)
	if err2 != nil {
		httpError := APIError{
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
		StreamUUID: streamPtr.UUID,
		Duration:   time.Since(startTime).Milliseconds(),
		Count:      int64(len(payload)),
		MessageIds: messageIds,
	}
	return c.Status(fiber.StatusAccepted).JSON(response)
}

func GetStreamUUIDFromParameter(c *fiber.Ctx) (stream.StreamUUID, *APIError) {
	streamUuid, err := uuid.Parse(c.Params("streamuuid"))
	if err != nil {
		// missing or invalid parameter
		vErr := ValidationError{FailedField: "streamuuid", Tag: "parameter", Value: c.Params("streamuuid")}
		return streamUuid, &APIError{
			Message:          "invalid stream uuid",
			Code:             constants.ErrorInvalidStreamUuid,
			HttpCode:         fiber.StatusBadRequest,
			ValidationErrors: []*ValidationError{&vErr},
			Err:              err,
		}
	}

	return streamUuid, nil
}

func GetStreamFromParameter(c *fiber.Ctx) (stream.StreamUUID, *stream.Stream, *APIError) {
	streamUuid, err := GetStreamUUIDFromParameter(c)
	if err != nil {
		return streamUuid, nil, err
	}

	streamPtr := stream.Streams.GetStream(streamUuid)
	if streamPtr == nil {
		// stream uuid not found among existing streams
		vErr := ValidationError{FailedField: "streamuuid", Tag: "parameter", Value: streamUuid.String()}
		return streamUuid, nil, &APIError{
			Message:          "stream not found",
			Code:             constants.ErrorStreamUuidNotFound,
			HttpCode:         fiber.StatusBadRequest,
			StreamUUID:       streamUuid,
			ValidationErrors: []*ValidationError{&vErr},
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
// @Success 200 {object} stream.StreamUUID "successful operation"
// @Success 500 {object} apierror.APIError
// @Router /api/v1/stream/index/{streamuuid}/rebuild [post]
func RebuildIndex(c *fiber.Ctx) error {
	_, streamPtr, apiErr := GetStreamFromParameter(c)
	if apiErr != nil {
		return apiErr.HTTPResponse(c)
	}

	indexStats, err := streamPtr.RebuildIndex()
	if err != nil {
		httpError := APIError{
			Message:    "cannot rebuild stream index",
			Details:    err.Error(),
			Code:       constants.ErrorCantRebuildStreamIndex,
			HttpCode:   fiber.StatusInternalServerError,
			StreamUUID: streamPtr.UUID,
			Err:        err,
		}
		return httpError.HTTPResponse(c)
	}

	account := account.GetAccount()
	log.Logger.Info(
		"Stream index rebuilt",
		zap.String("topic", "stream"),
		zap.String("method", "RebuildIndex"),
		zap.String("accountId", account.Id.String()),
		zap.String("ipAddress", c.IP()),
		zap.String("ipAddresses", strings.Join(c.IPs(), ";")),
		zap.String("streamUUID", streamPtr.UUID.String()),
	)

	return c.JSON(
		fiber.Map{
			"status": "success", "message": "Stream index rebuilt",
			"StreamUUID": streamPtr.UUID, "IndexStats": *indexStats,
		},
	)
}

func convertToProperties(propertiesMap map[string]string) *stream.StreamProperties {
	properties := stream.StreamProperties{}
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

func convertStreamListToJsonResult(rows *[]*stream.Stream) *JSONResultListStreamsProperties {
	r := make([]JSONResultListStreamsPropertiesResultRow, 0, len(*rows))
	for _, row := range *rows {
		r = append(
			r,
			JSONResultListStreamsPropertiesResultRow{
				FilePath:     row.FilePath,
				UUID:         row.UUID,
				CptMessages:  row.CptMessages,
				SizeInBytes:  row.SizeInBytes,
				CreationDate: row.CreationDate,
				LastUpdate:   row.LastUpdate,
				Properties:   row.Properties,
				LastMsgId:    row.LastMsgId,
			},
		)
	}

	return &JSONResultListStreamsProperties{
		Code: fiber.StatusOK,
		Result: &JSONResultListStreamsPropertiesResult{
			Total: len(*rows),
			Rows:  &r,
		},
	}
}
