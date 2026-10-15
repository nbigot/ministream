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
		// index of the last record (the first record when the stream is empty)
		if cptRecords > 0 {
			h.nextReadRecordIndex = cptRecords - 1
		} else {
			h.nextReadRecordIndex = 0
		}
	case "AFTER_LAST_MESSAGE":
		// index of the next record that will be added
		h.nextReadRecordIndex = cptRecords
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
	record, foundRecord, mayContinue := h.inMemoryStream.GetRecordAtIndex(h.nextReadRecordIndex)
	if !foundRecord {
		return 0, nil, false, mayContinue, nil
	}
	h.nextReadRecordIndex++
	// the result is the id of the record (not its index), and the record as generic
	// JSON values like the other storage providers (jq filters cannot be applied
	// on go structs nor on time.Time values)
	value := types.NewRecordValue(record.Id, record.CreationDate, record.Msg)
	return record.Id, value, true, mayContinue, nil
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
