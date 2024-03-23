package mysqlprovider

import (
	"database/sql"
	"sync"
	"time"

	"github.com/nbigot/ministream/types"

	"github.com/goccy/go-json"
	"go.uber.org/zap"
)

type StreamWriterMySQL struct {
	// implements IStreamWriter
	logger           *zap.Logger
	logVerbosity     int
	info             *types.StreamInfo
	schemaName       string
	catalogTableName string
	streamTableName  string
	pool             *sql.DB
	mu               sync.Mutex
}

func (w *StreamWriterMySQL) Init() error {
	return nil
}

func (w *StreamWriterMySQL) Open() error {
	return nil
}

func (w *StreamWriterMySQL) Close() error {
	w.logger.Debug(
		"Stream writer mysql closed",
		zap.String("topic", "streamwriter"),
		zap.String("method", "close"),
	)

	return nil
}

func (w *StreamWriterMySQL) Write(records *[]types.DeferedStreamRecord) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	cptRecords := uint64(len(*records))
	if cptRecords == 0 {
		return nil
	}

	if w.logVerbosity > 0 {
		w.logger.Debug(
			"write records into mysql",
			zap.String("topic", "stream"),
			zap.String("method", "Write"),
			zap.String("stream.uuid", w.info.UUID.String()),
			zap.Uint64("records.cpt", cptRecords),
		)
	}

	// process all records of the ingest buffer
	transaction, err := w.pool.Begin()
	if err != nil {
		w.logger.Error(
			"can't start transaction",
			zap.String("topic", "stream"),
			zap.String("method", "Write"),
			zap.String("stream.uuid", w.info.UUID.String()),
			zap.Error(err),
		)
		return err
	}

	var accumulatedSizeInBytes types.Size64 = 0
	var firstMsgTimestamp time.Time = w.info.ReadableMessages.FirstMsgTimestamp
	var lastMsgTimestamp time.Time = w.info.ReadableMessages.LastMsgTimestamp
	var lastMsgId types.MessageId
	var insertedRecords types.Size64 = 0
	var cptReadableRecords types.Size64 = w.info.ReadableMessages.CptMessages

	for _, record := range *records {
		if w.logVerbosity > 1 {
			w.logger.Debug(
				"write record into mysql",
				zap.String("topic", "stream"),
				zap.String("method", "Write"),
				zap.String("stream.uuid", w.info.UUID.String()),
				zap.Any("msg", record),
			)
		}

		// serialize the record into a string
		bytes, errMarshall := json.Marshal(record)
		if errMarshall != nil {
			w.logger.Error(
				"json",
				zap.String("topic", "stream"),
				zap.String("method", "Write"),
				zap.String("stream.uuid", w.info.UUID.String()),
				zap.Any("msg", record),
				zap.Error(errMarshall),
			)
			// drop the record (normally it should never happen, but never say never)
			return errMarshall
		}

		// insert the record into the sql table
		strjson := string(bytes)
		query := "INSERT INTO " + w.schemaName + "." + w.streamTableName + " (`id`, `timestamp`, `message`) VALUES (?, ?, ?)"
		_, err = transaction.Exec(query, record.Id, record.CreationDate, strjson)
		if err != nil {
			w.logger.Error(
				"can't insert record into stream",
				zap.String("topic", "stream"),
				zap.String("method", "Write"),
				zap.String("schema", w.schemaName),
				zap.String("table", w.streamTableName),
				zap.String("stream.uuid", w.info.UUID.String()),
				zap.Any("msg", record),
				zap.Error(err),
			)
			// rollback the transaction
			transaction.Rollback()
			return err
		}

		insertedRecords++
		accumulatedSizeInBytes += types.Size64(len(bytes))
		lastMsgTimestamp = record.CreationDate
		lastMsgId = record.Id
	}

	cptReadableRecords += insertedRecords

	if w.info.ReadableMessages.CptMessages == 0 {
		firstMsgTimestamp = (*records)[0].CreationDate
	}

	if err = w.SaveMetaInfo(transaction, cptReadableRecords, w.info.ReadableMessages.SizeInBytes+accumulatedSizeInBytes, w.info.IngestedMessages.FirstMsgId, lastMsgId, firstMsgTimestamp, lastMsgTimestamp); err != nil {
		return err
	}

	// commit the transaction
	err = transaction.Commit()
	if err != nil {
		w.logger.Error(
			"can't commit transaction",
			zap.String("topic", "stream"),
			zap.String("method", "Write"),
			zap.String("stream.uuid", w.info.UUID.String()),
			zap.Error(err),
		)
		// rollback the transaction
		_ = transaction.Rollback()
		return err
	}

	// update stream info
	// (update is done after the transaction commit to ensure that the data is really written)
	if w.info.ReadableMessages.CptMessages == 0 {
		// first message ever of the stream
		w.info.ReadableMessages.FirstMsgId = w.info.IngestedMessages.FirstMsgId
		w.info.ReadableMessages.LastMsgId = 0
		w.info.ReadableMessages.FirstMsgTimestamp = (*records)[0].CreationDate
	}

	w.info.ReadableMessages.CptMessages = cptReadableRecords
	w.info.ReadableMessages.SizeInBytes += accumulatedSizeInBytes
	w.info.ReadableMessages.LastMsgId = lastMsgId
	w.info.ReadableMessages.LastMsgTimestamp = lastMsgTimestamp

	if w.logVerbosity > 0 {
		w.logger.Debug(
			"finished to write records into mysql",
			zap.String("topic", "stream"),
			zap.String("method", "Write"),
			zap.String("stream.uuid", w.info.UUID.String()),
			zap.Uint64("records.cpt", cptRecords),
		)
	}

	return nil
}

func (w *StreamWriterMySQL) SaveMetaInfo(transaction *sql.Tx, cptMessages types.Size64, sizeInBytes types.Size64, firstMsgId types.MessageId, lastMsgId types.MessageId, firstMsgTimestamp time.Time, lastMsgTimestamp time.Time) error {
	streamUUID := w.info.UUID.String()
	if w.logVerbosity > 0 {
		w.logger.Debug(
			"saveMetaInfo",
			zap.String("topic", "stream"),
			zap.String("method", "SaveMetaInfo"),
			zap.String("stream.uuid", streamUUID),
		)
	}

	// implement SQL update
	query := "UPDATE " + w.schemaName + "." + w.catalogTableName + " SET `cache_cpt_rows`=?, `cache_size_in_bytes`=?, `cache_first_msg_id`=?, `cache_last_msg_id`=?, `cache_first_msg_timestamp`=?, `cache_last_msg_timestamp`=?, `last_update`=NOW() WHERE `id`=?"
	_, err := transaction.Exec(
		query,
		cptMessages,
		sizeInBytes,
		firstMsgId,
		lastMsgId,
		firstMsgTimestamp.Format(time.RFC3339),
		lastMsgTimestamp.Format(time.RFC3339),
		streamUUID,
	)
	if err != nil {
		w.logger.Error(
			"can't update stream",
			zap.String("topic", "stream"),
			zap.String("method", "SaveMetaInfo"),
			zap.String("schema", w.schemaName),
			zap.String("table", w.catalogTableName),
			zap.String("stream.uuid", streamUUID),
			zap.Error(err),
		)
		return err
	}

	return nil
}

func NewStreamWriterMySQL(info *types.StreamInfo, schemaName string, catalogTableName string, streamTableName string, pool *sql.DB, logger *zap.Logger, logVerbosity int) *StreamWriterMySQL {
	return &StreamWriterMySQL{
		logger:           logger,
		logVerbosity:     logVerbosity,
		info:             info,
		schemaName:       schemaName,
		catalogTableName: catalogTableName,
		streamTableName:  streamTableName,
		pool:             pool,
	}
}
