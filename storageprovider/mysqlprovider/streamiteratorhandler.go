package mysqlprovider

import (
	"database/sql"
	"encoding/json"
	"errors"
	"sync"

	"github.com/nbigot/ministream/types"

	"go.uber.org/zap"
)

type StreamIteratorHandlerMySQL struct {
	// implements IStreamIteratorHandler interface
	streamUUID       types.StreamUUID         // UUID of the stream
	itUUID           types.StreamIteratorUUID // UUID of the iterator
	initialized      bool                     // true if the iterator has been initialized
	nextRecordIdRead types.MessageId          // id of the next record to read from the iterator
	index            *StreamIndexMySQL        // index of the stream
	buffer           []rowMySQL               // buffer of records read from the SQL database
	bufferSize       int                      // number of records to read at once from the SQL database
	bufferIndex      int                      // index of the next record to read from the buffer
	bufferNextId     types.MessageId          // id of the next record to read from mysql into the buffer
	schemaName       string                   // name of the SQL schema holding the stream data
	streamTableName  string                   // name of the SQL table holding the stream data
	pool             *sql.DB                  // connection pool to the SQL database
	mu               sync.Mutex               // mutex to protect the buffer
	logger           *zap.Logger              // logger
}

type rowMySQL struct {
	Id           types.MessageId           `json:"i"`
	CreationDate string                    `json:"d"`
	Msg          types.DeferedStreamRecord `json:"m"`
}

func (h *StreamIteratorHandlerMySQL) Open() error {
	return nil
}

func (h *StreamIteratorHandlerMySQL) Close() error {
	return nil
}

func (h *StreamIteratorHandlerMySQL) Seek(request *types.StreamIteratorRequest) error {
	var (
		err                error
		nextRecordIdToRead types.MessageId
	)

	if h.initialized {
		return nil
	}

	switch request.IteratorType {
	case "FIRST_MESSAGE":
		nextRecordIdToRead, err = h.index.GetFirstMessageId()
	case "LAST_MESSAGE":
		nextRecordIdToRead, err = h.index.GetLastMessageId()
	case "AFTER_LAST_MESSAGE":
		nextRecordIdToRead, err = h.index.GetMessageIdAfterLastMessage()
	case "AT_MESSAGE_ID":
		nextRecordIdToRead, err = h.index.GetMessageId(request.MessageId)
	case "AFTER_MESSAGE_ID":
		nextRecordIdToRead, err = h.index.GetMessageIdAfterMessageId(request.MessageId)
	case "AT_TIMESTAMP":
		nextRecordIdToRead, err = h.index.GetMessageIdAtTimestamp(&request.Timestamp)
	default:
		nextRecordIdToRead = 0
		err = errors.New("invalid iterator type")
	}

	if err == nil {
		h.initialized = true
		h.nextRecordIdRead = nextRecordIdToRead
		h.bufferNextId = nextRecordIdToRead
		h.bufferIndex = 0
	}

	return err
}

func (h *StreamIteratorHandlerMySQL) SaveSeek() error {
	return nil
}

func (h *StreamIteratorHandlerMySQL) GetNextRecord() (types.MessageId, interface{}, bool, bool, error) {
	// get the next record of the stream.
	var err error

	// if nothing left to read from the buffer then read another chunk of records from the SQL database
	if h.bufferIndex >= len(h.buffer) {
		err = h.fillBuffer()
		if err != nil {
			// result is: (no record, not record found, may continue, error)
			return 0, nil, false, true, err
		}
		h.bufferIndex = 0
	}

	// if the buffer is empty after reading from the SQL database, return no record found
	if len(h.buffer) == 0 {
		// result is: (no record, not record found, may continue, no error)
		return 0, nil, false, true, nil
	}

	// return the next record from the buffer
	record := h.buffer[h.bufferIndex]
	h.bufferIndex++
	h.nextRecordIdRead = record.Id + 1

	// result is: (valid record, record found, may continue, no error)
	return record.Id, record.Msg, true, true, nil
}

func (h *StreamIteratorHandlerMySQL) fillBuffer() error {
	// read-ahead optimization:
	// read a chunk of records from the SQL steam table
	h.mu.Lock()
	defer h.mu.Unlock()

	query := "SELECT `id`, `timestamp`, `message` FROM " + h.schemaName + "." + h.streamTableName + " WHERE `id` >= ? ORDER BY `id` ASC LIMIT ?"
	rows, err := h.pool.Query(query, h.bufferNextId, h.bufferSize)
	if err != nil {
		h.logger.Error(
			"Error while reading records from the SQL database",
			zap.String("topic", "streamiterator"),
			zap.String("method", "fillBuffer"),
			zap.String("schema", h.schemaName),
			zap.String("table", h.streamTableName),
			zap.Error(err),
		)
		return err
	}
	defer rows.Close()

	// reset the buffer but keep the capacity
	h.buffer = h.buffer[:0]

	// read the records
	var strMsg string
	for rows.Next() {
		row := rowMySQL{}
		if err := rows.Scan(
			&row.Id,
			&row.CreationDate,
			&strMsg,
		); err != nil {
			h.logger.Error(
				"Can't read stream",
				zap.String("topic", "streamiterator"),
				zap.String("method", "fillBuffer"),
				zap.String("schema", h.schemaName),
				zap.String("table", h.streamTableName),
				zap.Error(err),
			)
			return err
		}

		// unmarshal properties from JSON
		if err := json.Unmarshal([]byte(strMsg), &row.Msg); err != nil {
			h.logger.Error(
				"Can't unmarshal message from JSON",
				zap.String("topic", "streamiterator"),
				zap.String("method", "fillBuffer"),
				zap.String("schema", h.schemaName),
				zap.String("table", h.streamTableName),
				zap.Uint64("message.id", row.Id),
				zap.Error(err),
			)
			// drop the record
			continue
		}

		h.buffer = append(h.buffer, row)
		h.bufferNextId = row.Id + 1
	}

	return nil
}

func NewStreamIteratorHandlerMySQL(streamUUID types.StreamUUID, iteratorUUID types.StreamIteratorUUID, idx *StreamIndexMySQL, schemaName string, streamTableName string, pool *sql.DB, bufferSize int, logger *zap.Logger) *StreamIteratorHandlerMySQL {
	return &StreamIteratorHandlerMySQL{
		streamUUID:       streamUUID,
		itUUID:           iteratorUUID,
		initialized:      false,
		nextRecordIdRead: 0,
		index:            idx,
		bufferSize:       bufferSize,
		buffer:           make([]rowMySQL, 0, bufferSize),
		bufferIndex:      0,
		schemaName:       schemaName,
		streamTableName:  streamTableName,
		pool:             pool,
		logger:           logger,
	}
}
