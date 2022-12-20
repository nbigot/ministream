package inmemoryprovider

import (
	"bufio"
	"errors"

	"github.com/nbigot/ministream/types"

	"go.uber.org/zap"
)

type StreamIteratorHandlerInMemory struct {
	// implements IStreamIteratorHandler interface
	streamUUID          types.StreamUUID
	itUUID              types.StreamIteratorUUID
	initialized         bool
	inMemoryStream      *InMemoryStream
	nextReadRecordIndex types.MessageId
	reader              *bufio.Reader
	logger              *zap.Logger
}

func (h *StreamIteratorHandlerInMemory) Open() error {
	h.nextReadRecordIndex = 0
	return nil
}

func (h *StreamIteratorHandlerInMemory) Close() error {
	return nil
}

func (h *StreamIteratorHandlerInMemory) Seek(request *types.StreamIteratorRequest) error {
	var err error = nil

	if h.initialized {
		return nil
	}

	cptRecords := h.inMemoryStream.GetRecordsCount()

	switch request.IteratorType {
	case "FIRST_MESSAGE":
		h.nextReadRecordIndex = 0
	case "LAST_MESSAGE":
		h.nextReadRecordIndex = cptRecords
	case "AFTER_LAST_MESSAGE":
		h.nextReadRecordIndex = cptRecords + 1
	case "AT_MESSAGE_ID":
		h.nextReadRecordIndex, err = h.inMemoryStream.GetIndexAtMessageId(request.MessageId)
	case "AFTER_MESSAGE_ID":
		h.nextReadRecordIndex, err = h.inMemoryStream.GetIndexAfterMessageId(request.MessageId)
	case "AT_TIMESTAMP":
		h.nextReadRecordIndex, err = h.inMemoryStream.GetIndexAtTimestamp(&request.Timestamp)
	default:
		h.nextReadRecordIndex = 0
		err = errors.New("invalid iterator type")
	}

	if err == nil {
		h.initialized = true
	}

	return err
}

func (h *StreamIteratorHandlerInMemory) SaveSeek() error {
	// in memory stream iterator is not persisted,
	// therefore: nothing to do
	return nil
}

func (h *StreamIteratorHandlerInMemory) GetNextRecord() (types.MessageId, interface{}, bool, bool, error) {
	recordIndex := h.nextReadRecordIndex
	record, foundRecord, mayContinue := h.inMemoryStream.GetRecordAtIndex(recordIndex)
	if foundRecord {
		h.nextReadRecordIndex++
	}
	return recordIndex, record, foundRecord, mayContinue, nil
}

func NewStreamIteratorHandlerInMemory(streamUUID types.StreamUUID, iteratorUUID types.StreamIteratorUUID, inMemoryStream *InMemoryStream, logger *zap.Logger) *StreamIteratorHandlerInMemory {
	return &StreamIteratorHandlerInMemory{
		streamUUID:          streamUUID,
		itUUID:              iteratorUUID,
		initialized:         false,
		inMemoryStream:      inMemoryStream,
		nextReadRecordIndex: 0,
		reader:              nil,
		logger:              logger,
	}
}
