package stream

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	"github.com/goccy/go-json"
	"github.com/google/uuid"
	"github.com/itchyny/gojq"
	"github.com/qri-io/jsonschema"
)

type StreamIteratorUUID = uuid.UUID
type StreamIteratorMap = map[StreamIteratorUUID]*StreamIterator

type StreamIteratorRequest struct {
	IteratorType string    `json:"iteratorType" validate:"required,oneof=FIRST_MESSAGE LAST_MESSAGE AFTER_LAST_MESSAGE AT_MESSAGE_ID AFTER_MESSAGE_ID AT_TIMESTAMP"`
	MessageId    MessageId `json:"messageId"`
	Timestamp    time.Time `json:"timestamp"`
	JqFilter     string    `json:"jqFilter"`
	Name         string    `json:"name"`
}

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
	UUID              StreamIteratorUUID
	request           *StreamIteratorRequest
	jqFilter          *gojq.Query
	file              *os.File
	filename          string
	FileOffset        int64
	initialized       bool
	LastMessageIdRead MessageId
	Stats             StreamIteratorStats
	// TODO: add timeout (self delete at timeout)
}

var rs = jsonschema.Schema{}

func (it *StreamIterator) Open() error {
	if it.filename == "" {
		return errors.New("empty stream filename")
	}

	if it.file != nil {
		it.file.Close()
	}

	var err error
	it.file, err = os.Open(it.filename)
	if err != nil {
		return err
	}

	return nil
}

func (it *StreamIterator) Close() {
	it.request = nil
	it.jqFilter = nil
	if it.file != nil {
		it.file.Close()
		it.file = nil
	}
}

func (it *StreamIterator) Seek(idx *StreamIndex) error {
	var err error
	if it.initialized {
		_, err = it.file.Seek(it.FileOffset, io.SeekStart)
		return err
	}

	switch it.request.IteratorType {
	case "FIRST_MESSAGE":
		it.FileOffset, err = idx.GetOffsetFirstMessage()
	case "LAST_MESSAGE":
		it.FileOffset, err = idx.GetOffsetLastMessage()
	case "AFTER_LAST_MESSAGE":
		it.FileOffset, err = idx.GetOffsetAfterLastMessage()
	case "AT_MESSAGE_ID":
		it.FileOffset, err = idx.GetOffsetAtMessageId(it.request.MessageId)
	case "AFTER_MESSAGE_ID":
		it.FileOffset, err = idx.GetOffsetAfterMessageId(it.request.MessageId)
	case "AT_TIMESTAMP":
		it.FileOffset, err = idx.GetOffsetAtTimestamp(&it.request.Timestamp)
	default:
		it.FileOffset = 0
		err = errors.New("invalid iterator type")
	}

	if err == nil {
		it.initialized = true
		_, err = it.file.Seek(it.FileOffset, io.SeekStart)
	}

	return err
}

func (it *StreamIterator) SaveSeek() error {
	var err error
	it.FileOffset, err = it.file.Seek(0, io.SeekCurrent)
	return err
}

func CreateRecordsIterator(r *StreamIteratorRequest) (*StreamIterator, error) {
	var jqFilter *gojq.Query = nil
	if r.JqFilter != "" {
		var errJq error
		jqFilter, errJq = gojq.Parse(r.JqFilter)
		if errJq != nil {
			return nil, errJq
		}
	}

	it := StreamIterator{
		UUID:     uuid.New(),
		request:  r,
		jqFilter: jqFilter,
		file:     nil,
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
