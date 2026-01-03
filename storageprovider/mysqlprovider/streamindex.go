package mysqlprovider

import (
	"database/sql"
	"errors"
	"sync"
	"time"

	"github.com/nbigot/ministream/types"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type StreamIndexMySQL struct {
	streamUUID   uuid.UUID
	info         *types.StreamInfo
	logger       *zap.Logger
	logVerbosity int
	schemaName   string // name of the SQL schema holding the catalog of streams
	tableName    string // name of the SQL table holding the stream data
	pool         *sql.DB
	mu           sync.Mutex
}

type StreamIndexStats struct {
	CptMessages       int64
	FirstMsgId        types.MessageId
	LastMsgId         types.MessageId
	FirstMsgTimestamp time.Time
	LastMsgTimestamp  time.Time
}

func (idx *StreamIndexMySQL) Close() error {
	return nil
}

func (idx *StreamIndexMySQL) BuildIndex() (*StreamIndexStats, error) {
	// Build or rebuild index
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.logger.Info(
		"Build index started",
		zap.String("topic", "index"),
		zap.String("method", "BuildIndex"),
		zap.String("stream.uuid", idx.streamUUID.String()),
	)

	fullTableName := idx.GetFullTableName()

	// rebuild/optimize mysql index (optional but recommended for performance)
	query := "OPTIMIZE TABLE " + fullTableName
	_, err := idx.pool.Exec(query)
	if err != nil {
		idx.logger.Error(
			"Error while building index",
			zap.String("topic", "index"),
			zap.String("method", "BuildIndex"),
			zap.String("stream.uuid", idx.streamUUID.String()),
			zap.Error(err),
		)
		return nil, err
	}

	var stats *StreamIndexStats
	if stats, err = idx.GetStats(); err != nil {
		return nil, err
	}

	idx.logger.Info(
		"Build index ended",
		zap.String("topic", "index"),
		zap.String("method", "BuildIndex"),
		zap.String("stream.uuid", idx.streamUUID.String()),
		zap.Int64("index.rowsCount", stats.CptMessages),
		zap.Uint64("index.firstMsgId", stats.FirstMsgId),
		zap.Uint64("index.lastMsgId", stats.LastMsgId),
		zap.Time("index.firstMsgTimestamp", stats.FirstMsgTimestamp),
		zap.Time("index.lastMsgTimestamp", stats.LastMsgTimestamp),
	)

	return stats, nil
}

func (idx *StreamIndexMySQL) GetStats() (*StreamIndexStats, error) {

	// compute stats
	stats := StreamIndexStats{}
	fullTableName := idx.GetFullTableName()

	// count messages in the stream
	query := "SELECT COUNT(*) FROM " + fullTableName
	row := idx.pool.QueryRow(query)
	err := row.Scan(&stats.CptMessages)
	if err != nil {
		idx.logger.Error(
			"Error while building index",
			zap.String("topic", "index"),
			zap.String("method", "BuildIndex"),
			zap.String("stream.uuid", idx.streamUUID.String()),
			zap.Error(err),
		)
		return nil, err
	}

	// get first message id and timestamp
	query = "SELECT `id`, `creation_date` FROM " + fullTableName + " ORDER BY `id` ASC LIMIT 1"
	row = idx.pool.QueryRow(query)
	err = row.Scan(&stats.FirstMsgId, &stats.FirstMsgTimestamp)
	if err != nil {
		idx.logger.Error(
			"Error while building index",
			zap.String("topic", "index"),
			zap.String("method", "BuildIndex"),
			zap.String("stream.uuid", idx.streamUUID.String()),
			zap.Error(err),
		)
		return nil, err
	}

	// get last message id and timestamp
	query = "SELECT `id`, `creation_date` FROM " + fullTableName + " ORDER BY `id` DESC LIMIT 1"
	row = idx.pool.QueryRow(query)
	err = row.Scan(&stats.LastMsgId, &stats.LastMsgTimestamp)
	if err != nil {
		idx.logger.Error(
			"Error while building index",
			zap.String("topic", "index"),
			zap.String("method", "BuildIndex"),
			zap.String("stream.uuid", idx.streamUUID.String()),
			zap.Error(err),
		)
		return nil, err
	}

	return &stats, nil
}

func (idx *StreamIndexMySQL) GetFullTableName() string {
	return idx.schemaName + ".stream_" + idx.streamUUID.String()
}

func (idx *StreamIndexMySQL) GetFirstMessageId() (types.MessageId, error) {
	return idx.info.ReadableMessages.FirstMsgId, nil
}

func (idx *StreamIndexMySQL) GetLastMessageId() (types.MessageId, error) {
	return idx.info.ReadableMessages.LastMsgId, nil
}

func (idx *StreamIndexMySQL) GetMessageIdAfterLastMessage() (types.MessageId, error) {
	if idx.info.ReadableMessages.CptMessages == 0 {
		return 0, nil
	}

	return idx.info.ReadableMessages.LastMsgId + 1, nil
}

func (idx *StreamIndexMySQL) GetMessageId(messageId types.MessageId) (types.MessageId, error) {
	if messageId > idx.info.ReadableMessages.LastMsgId {
		return 0, errors.New("message id not found")
	}

	// find the message id
	fullTableName := idx.GetFullTableName()
	query := "SELECT EXISTS(SELECT 1 FROM " + fullTableName + " WHERE `id` = ?)"
	row := idx.pool.QueryRow(query, messageId)
	var exists int
	err := row.Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			// no message id found after the given message id
			return 0, errors.New("message id not found")
		}

		idx.logger.Error(
			"Error while looking for message id",
			zap.String("topic", "index"),
			zap.String("method", "GetMessageId"),
			zap.String("stream.uuid", idx.streamUUID.String()),
			zap.Uint64("message.id", messageId),
			zap.Error(err),
		)
		return 0, err
	}

	if exists == 0 {
		return 0, errors.New("message id not found")
	}

	return messageId, nil
}

func (idx *StreamIndexMySQL) GetMessageIdAfterMessageId(messageId types.MessageId) (types.MessageId, error) {
	if messageId > idx.info.ReadableMessages.LastMsgId {
		return 0, errors.New("message id not found")
	}

	// find the message id after the given message id
	fullTableName := idx.GetFullTableName()
	query := "SELECT `id` FROM " + fullTableName + " WHERE `id` > ? ORDER BY `id` ASC LIMIT 1"
	row := idx.pool.QueryRow(query, messageId)
	var nextMessageId types.MessageId
	err := row.Scan(&nextMessageId)
	if err != nil {
		if err == sql.ErrNoRows {
			// no message id found after the given message id
			return 0, errors.New("no message found")
		}

		idx.logger.Error(
			"Error while looking for message id",
			zap.String("topic", "index"),
			zap.String("method", "GetMessageIdAfterMessageId"),
			zap.String("stream.uuid", idx.streamUUID.String()),
			zap.Uint64("message.id", messageId),
			zap.Error(err),
		)
		return 0, err
	}

	return nextMessageId, nil
}

func (idx *StreamIndexMySQL) GetMessageIdAtTimestamp(timestamp *time.Time) (types.MessageId, error) {
	// find the message id after the given timestamp
	fullTableName := idx.GetFullTableName()
	query := "SELECT `id` FROM " + fullTableName + " WHERE `timestamp` >= ? ORDER BY `id` ASC LIMIT 1"
	row := idx.pool.QueryRow(query, timestamp)
	var nextMessageId types.MessageId
	err := row.Scan(&nextMessageId)
	if err != nil {
		if err == sql.ErrNoRows {
			// no message found at the exact timestamp or after the given timestamp
			return 0, errors.New("no message found")
		}

		idx.logger.Error(
			"Error while looking for timestamp",
			zap.String("topic", "index"),
			zap.String("method", "GetMessageIdAtTimestamp"),
			zap.String("stream.uuid", idx.streamUUID.String()),
			zap.Error(err),
		)
		return 0, err
	}

	return nextMessageId, nil
}

func (idx *StreamIndexMySQL) Log() {
	idx.logger.Info(
		"StreamIndex",
		zap.String("topic", "index"),
		zap.String("method", "Log"),
		zap.String("stream.uuid", idx.streamUUID.String()),
	)
}

func NewStreamIndex(streamUUID uuid.UUID, info *types.StreamInfo, schemaName string, tableName string, pool *sql.DB, logger *zap.Logger) *StreamIndexMySQL {
	return &StreamIndexMySQL{
		streamUUID:   streamUUID,
		info:         info,
		logger:       logger,
		logVerbosity: 0,
		schemaName:   schemaName,
		tableName:    tableName,
		pool:         pool,
	}
}
