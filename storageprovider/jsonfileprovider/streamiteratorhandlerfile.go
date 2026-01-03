package jsonfileprovider

import (
	"bufio"
	"errors"
	"io"
	"os"

	"github.com/nbigot/ministream/types"

	"github.com/goccy/go-json"
	"go.uber.org/zap"
)

const EOLChar = '\n'

type StreamIteratorHandlerFile struct {
	// implements IStreamIteratorHandler interface
	streamUUID       types.StreamUUID
	itUUID           types.StreamIteratorUUID
	initialized      bool
	file             *os.File
	filename         string
	FileOffset       int64
	bytesRead        int64
	nextRecordIdRead types.MessageId
	index            *StreamIndexFile
	reader           *bufio.Reader
	logger           *zap.Logger
}

func (h *StreamIteratorHandlerFile) Open() error {
	// open the json data file
	if h.filename == "" {
		return errors.New("empty stream filename")
	}

	if h.file != nil {
		_ = h.file.Close()
	}

	var err error
	h.file, err = os.Open(h.filename)
	if err != nil {
		return err
	}

	return nil
}

func (h *StreamIteratorHandlerFile) Close() error {
	if h.file != nil {
		_ = h.file.Close()
		h.file = nil
	}

	if h.index != nil {
		_ = h.index.Close()
		h.index = nil
	}

	return nil
}

func (h *StreamIteratorHandlerFile) Seek(request *types.StreamIteratorRequest) error {
	var (
		err                error
		nextRecordIdToRead types.MessageId
	)
	if h.initialized {
		_, err = h.file.Seek(h.FileOffset, io.SeekStart)
		return err
	}

	switch request.IteratorType {
	case "FIRST_MESSAGE":
		nextRecordIdToRead, h.FileOffset, err = h.index.GetOffsetFirstMessage()
	case "LAST_MESSAGE":
		nextRecordIdToRead, h.FileOffset, err = h.index.GetOffsetLastMessage()
	case "AFTER_LAST_MESSAGE":
		nextRecordIdToRead, h.FileOffset, err = h.index.GetOffsetAfterLastMessage()
	case "AT_MESSAGE_ID":
		nextRecordIdToRead, h.FileOffset, err = h.index.GetOffsetAtMessageId(request.MessageId)
	case "AFTER_MESSAGE_ID":
		nextRecordIdToRead, h.FileOffset, err = h.index.GetOffsetAfterMessageId(request.MessageId)
	case "AT_TIMESTAMP":
		nextRecordIdToRead, h.FileOffset, err = h.index.GetOffsetAtTimestamp(&request.Timestamp)
	default:
		h.FileOffset = 0
		nextRecordIdToRead = 0
		err = errors.New("invalid iterator type")
	}

	if err == nil {
		if _, err = h.file.Seek(h.FileOffset, io.SeekStart); err != nil {
			return err
		}
		h.initialized = true
		h.reader = bufio.NewReaderSize(h.file, 1024*1024)
		h.reader.Reset(h.file)
		h.nextRecordIdRead = nextRecordIdToRead
	}

	return err
}

func (h *StreamIteratorHandlerFile) SaveSeek() error {
	var err error
	h.FileOffset, err = h.file.Seek(0, io.SeekCurrent)
	return err
}

func (h *StreamIteratorHandlerFile) GetNextRecord() (types.MessageId, interface{}, bool, bool, error) {
	line, errRead := h.reader.ReadString(EOLChar)
	if errRead != nil {
		// err is often io.EOF (end of file reached)
		// err may also raise when EOL char was not found
		errSeek := h.SaveSeek()
		if errSeek != nil {
			// result is: (no record, no record found, cannot continue, error)
			return 0, nil, false, false, errSeek
		}

		if errRead == io.EOF {
			// result is: (no record, no record found, cannot continue, no error)
			return 0, nil, false, false, nil
		} else {
			h.logger.Error(
				"cannot read record line",
				zap.String("topic", "streamiterator"),
				zap.String("method", "GetNextRecord"),
				zap.String("stream.uuid", h.streamUUID.String()),
				zap.String("it.uuid", h.itUUID.String()),
				zap.String("line", line),
				zap.Error(errRead),
			)
			// result is: (no record, no record found (or cannot read it), cannot continue, error)
			return 0, nil, true, false, errRead
		}
	}

	h.bytesRead += int64(len(line))
	lastRecordIdRead := h.nextRecordIdRead
	h.nextRecordIdRead++

	var message interface{}
	if errUnmarshal := json.Unmarshal([]byte(line), &message); errUnmarshal != nil {
		h.logger.Error(
			"json format error",
			zap.String("topic", "streamiterator"),
			zap.String("method", "GetNextRecord"),
			zap.String("stream.uuid", h.streamUUID.String()),
			zap.String("it.uuid", h.itUUID.String()),
			zap.String("line", line),
			zap.Error(errUnmarshal),
		)
		// result is: (no record, record found, may continue, error)
		return lastRecordIdRead, nil, true, true, errUnmarshal
	}

	// result is: (valid record, record found, may continue, no error)
	return lastRecordIdRead, message, true, true, nil
}

func NewStreamIteratorHandlerFile(streamUUID types.StreamUUID, iteratorUUID types.StreamIteratorUUID, filename string, idx *StreamIndexFile, logger *zap.Logger) *StreamIteratorHandlerFile {
	return &StreamIteratorHandlerFile{
		streamUUID:       streamUUID,
		itUUID:           iteratorUUID,
		initialized:      false,
		file:             nil,
		filename:         filename,
		FileOffset:       0,
		bytesRead:        0,
		nextRecordIdRead: 0,
		index:            idx,
		reader:           nil,
		logger:           logger,
	}
}
