package types

import "time"

type StreamIteratorRequest struct {
	IteratorType string    `json:"iteratorType" validate:"required,oneof=FIRST_MESSAGE LAST_MESSAGE AFTER_LAST_MESSAGE AT_MESSAGE_ID AFTER_MESSAGE_ID AT_TIMESTAMP"`
	MessageId    MessageId `json:"messageId"`
	Timestamp    time.Time `json:"timestamp"`
	JqFilter     string    `json:"jqFilter"`
	Name         string    `json:"name"`
}

type IStreamIteratorHandler interface {
	Open() error
	Close() error
	Seek(request *StreamIteratorRequest) error
	SaveSeek() error
	GetNextRecord() (MessageId, interface{}, bool, bool, error)
}
