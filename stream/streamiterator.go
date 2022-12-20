package stream

import (
	"context"
	"errors"
	"time"

	"github.com/nbigot/ministream/types"
	. "github.com/nbigot/ministream/types"

	"github.com/goccy/go-json"
	"github.com/itchyny/gojq"
	"github.com/qri-io/jsonschema"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

type StreamIteratorMap = map[StreamIteratorUUID]*StreamIterator

// statistics of an iterator
type StreamIteratorStats struct {
	BytesRead      int64
	RecordsRead    int64
	RecordsErrors  int64
	RecordsSkipped int64
	RecordsSent    int64
	LastTimeRead   time.Time
}

type StreamIterator struct {
	streamUUID       StreamUUID
	itUUID           StreamIteratorUUID
	request          *StreamIteratorRequest
	jqFilter         *gojq.Query
	LastRecordIdRead MessageId
	Stats            StreamIteratorStats
	handler          IStreamIteratorHandler
	logger           *zap.Logger
	// TODO: add timeout (self delete at timeout)
}

var rs = jsonschema.Schema{}

func (it *StreamIterator) GetUUID() StreamIteratorUUID {
	return it.itUUID
}

func (it *StreamIterator) GetName() string {
	if it.request != nil {
		return it.request.Name
	} else {
		return ""
	}
}

func (it *StreamIterator) Open() error {
	return it.handler.Open()
}

func (it *StreamIterator) Close() error {
	it.request = nil
	it.jqFilter = nil
	return it.handler.Close()
}

func (it *StreamIterator) Seek() error {
	return it.handler.Seek(it.request)
}

func (it *StreamIterator) SaveSeek() error {
	return it.handler.SaveSeek()
}

func (it *StreamIterator) GetRecords(c *fasthttp.RequestCtx, maxRecords uint) (*GetStreamRecordsResponse, error) {
	var err error
	startTime := time.Now()

	response := GetStreamRecordsResponse{
		Status:             "",
		Duration:           0,
		Count:              0,
		CountErrors:        0,
		CountSkipped:       0,
		LastRecordIdRead:   0,
		Remain:             false,
		StreamUUID:         it.streamUUID,
		StreamIteratorUUID: it.itUUID,
		Records:            make([]interface{}, 0),
	}

	defer func() {
		response.Duration = time.Since(startTime).Milliseconds()
	}()

	if err = it.Seek(); err != nil {
		response.Status = "error"
		return &response, err
	}

	it.Stats.LastTimeRead = time.Now()

	var (
		record                interface{}
		recordId              types.MessageId
		lastRecordIdProcessed types.MessageId
		foundRecord           bool
		canContinue           bool
	)

	for {
		recordId, record, foundRecord, canContinue, err = it.handler.GetNextRecord()

		if !foundRecord {
			// no record found, this is the end of the stream
			err = nil
			break
		}

		lastRecordIdProcessed = recordId

		if err != nil {
			response.CountErrors += 1
			if canContinue {
				continue
			} else {
				// non recoverable error, cannot simply skip this record
				break
			}
		}

		it.Stats.RecordsRead++
		//it.Stats.BytesRead += int64(len(line))

		if it.jqFilter == nil {
			response.Count += 1
			response.Records = append(response.Records, record)
		} else {
			// apply filter on message

			// bash example: echo {"foo": 0} | jq .foo
			// bash example: echo [{"foo": 0}] | jq .[0]
			// bash example: echo [{"foo": 0}] | jq .[0].foo
			// ".foo | .."
			// TODO: iterator checkpoint?
			// TODO: save iterator last seek file?

			jqIter := it.jqFilter.RunWithContext(c, record)
			v, ok := jqIter.Next()
			if ok {
				// the message is matching the jq filter
				response.Count += 1
				response.Records = append(response.Records, v)
			} else {
				if err, isAnError := v.(error); isAnError {
					// invalid (TODO: decide to keep or to skip the message)
					it.logger.Error(
						"jq error",
						zap.String("topic", "stream"),
						zap.String("method", "GetRecords"),
						zap.String("stream.uuid", it.streamUUID.String()),
						zap.String("jq", it.jqFilter.String()),
						zap.Error(err),
					)
					response.CountErrors += 1
				} else {
					// does not match the jq filter therefore skip the message
					response.CountSkipped += 1
				}
			}
		}

		if uint(len(response.Records)) >= maxRecords {
			// reach the maximum allowed records count by response
			response.Remain = true
			err = nil
			break
		}
	}

	if err != nil {
		response.Status = "error"
		return &response, err
	}

	it.LastRecordIdRead = lastRecordIdProcessed
	it.Stats.RecordsErrors += response.CountErrors
	it.Stats.RecordsSkipped += response.CountSkipped
	it.Stats.RecordsSent += response.Count

	response.LastRecordIdRead = lastRecordIdProcessed

	if err = it.SaveSeek(); err != nil {
		response.Status = "error"
		return &response, err
	}

	response.Status = "success"
	return &response, nil
}

func NewStreamIterator(streamUUID StreamUUID, iteratorUUID StreamIteratorUUID, r *StreamIteratorRequest, handler IStreamIteratorHandler, logger *zap.Logger) (*StreamIterator, error) {
	var jqFilter *gojq.Query = nil
	if r.JqFilter != "" {
		var errJq error
		jqFilter, errJq = gojq.Parse(r.JqFilter)
		if errJq != nil {
			return nil, errJq
		}
	}

	it := StreamIterator{
		streamUUID: streamUUID,
		itUUID:     iteratorUUID,
		request:    r,
		jqFilter:   jqFilter,
		handler:    handler,
		logger:     logger,
	}

	return &it, nil
}

func ValidateStreamIteratorRequest(ctx context.Context, data []byte) error {
	errs, err := rs.ValidateBytes(ctx, data)
	if err != nil {
		return err
	}

	if len(errs) > 0 {
		strErr := ""
		for i, kerr := range errs {
			if i > 0 {
				strErr += ", "
			}
			strErr += kerr.Error()
		}
		return errors.New(strErr)
	}

	return nil
}

func init() {
	var schemaData = []byte(`{
    "$id": "https://qri.io/schema/",
		"title": "StreamIteratorRequest",
		"description": "schema validating StreamIteratorRequest",
		"type": "object",
		"oneOf": [
			{
				"type": "object",
				"properties": {
						"iteratorType": {
								"enum": ["FIRST_MESSAGE", "LAST_MESSAGE", "AFTER_LAST_MESSAGE"]
						},
						"jqFilter": {
								"type": "string",
								"minLength": 1,
								"maxLength": 512
						},
						"name": {
								"type": "string",
								"minLength": 1,
								"maxLength": 256
						}
					},
					"required": ["iteratorType"],
					"additionalProperties": false
			},
			{
				"type": "object",
				"properties": {
						"iteratorType": {
								"enum": ["AT_MESSAGE_ID", "AFTER_MESSAGE_ID"]
						},
						"messageId": {
								"type": "integer",
								"minimum": 0
						},
						"jqFilter": {
								"type": "string",
								"minLength": 1,
								"maxLength": 512
						},
						"name": {
								"type": "string",
								"minLength": 1,
								"maxLength": 256
						}
					},
					"required": ["iteratorType", "messageId"],
					"additionalProperties": false
			},
			{
				"type": "object",
				"properties": {
						"iteratorType": {
								"const": "AT_TIMESTAMP"
						},
						"timestamp": {
								"type": "string",
								"pattern": "^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$"
						},
						"jqFilter": {
								"type": "string",
								"minLength": 1,
								"maxLength": 512
						},
						"name": {
								"type": "string",
								"minLength": 1,
								"maxLength": 256
						}
					},
					"required": ["iteratorType", "timestamp"],
					"additionalProperties": false
			}
		]
	}`)

	if err := json.Unmarshal(schemaData, &rs); err != nil {
		panic("unmarshal schema: " + err.Error())
	}
}

/*
Valid iterator request examples:

{
	"iteratorType": "FIRST_MESSAGE"
}

{
	"iteratorType": "LAST_MESSAGE"
}

{
	"iteratorType": "AFTER_LAST_MESSAGE"
}

{
	"iteratorType": "AT_MESSAGE_ID",
  "messageId": 1234
}

{
	"iteratorType": "AFTER_MESSAGE_ID",
  "messageId": 1234
}

{
	"iteratorType": "AT_TIMESTAMP",
	"timestamp": "2006-01-02T15:04:06Z"
}

{
	"iteratorType": "FIRST_MESSAGE"
	"Name": "myApp"
}

{
	"iteratorType": "FIRST_MESSAGE"
	"jqFilter": ".[0]"
}
*/
