package mysqlprovider

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"encoding/json"

	"github.com/nbigot/ministream/types"

	"go.uber.org/zap"
)

type StreamCatalogMySQL struct {
	// implements IStorageCatalog interface
	logger            *zap.Logger
	pool              *sql.DB
	mu                sync.Mutex
	schemaName        string               // name of the SQL schema holding the catalog of streams
	catalogTableName  string               // name of the SQL table storing the catalog of streams
	streamTablePrefix string               // prefix for the stream tables
	streams           types.StreamInfoDict // map of streams indexed by UUID
}

func (s *StreamCatalogMySQL) Init() error {
	// empty the streams list
	s.streams = make(types.StreamInfoDict)
	return s.EnsureCatalogExists()
}

func (s *StreamCatalogMySQL) Stop() error {
	return nil
}

func (s *StreamCatalogMySQL) EnsureCatalogExists() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// check if the SQL schema holding catalog of streams exists
	exists, err := s.SchemaExists()
	if err != nil {
		return err
	}

	if !exists {
		// if schema does not exist therefore create it
		if err := s.CreateSchema(); err != nil {
			return err
		}
	}

	// check if the SQL table holding catalog of streams
	exists, err = s.CatalogExists()
	if err != nil {
		return err
	}

	if exists {
		// if catalog already exists therefore do nothing
		return nil
	}

	// create new empty catalog of streams
	return s.CreateEmptyCatalog()
}

func (s *StreamCatalogMySQL) SchemaExists() (bool, error) {
	var err error
	var name string

	// check if the SQL schema holding catalog of streams exists
	query := "SHOW SCHEMAS LIKE '" + s.schemaName + "'"
	db_row := s.pool.QueryRow(query)
	if err = db_row.Scan(&name); err != nil {
		if err == sql.ErrNoRows {
			// schema does not exist
			return false, nil
		}

		s.logger.Fatal(
			"Can't check if schema exists",
			zap.String("topic", "stream"),
			zap.String("method", "CatalogExists"),
			zap.String("schema", s.schemaName),
			zap.Error(err),
		)
		return false, err
	}

	// schema exists
	return true, nil
}

func (s *StreamCatalogMySQL) CreateSchema() error {
	// create new SQL schema that will hold the catalog of streams
	query := "CREATE SCHEMA " + s.schemaName
	_, err := s.pool.Exec(query)
	if err != nil {
		s.logger.Fatal(
			"Can't create schema",
			zap.String("topic", "stream"),
			zap.String("method", "CreateSchema"),
			zap.String("schema", s.schemaName),
			zap.Error(err),
		)
		return err
	}

	return nil
}

func (s *StreamCatalogMySQL) CatalogExists() (bool, error) {
	var err error
	var name string

	// check if the SQL table holding catalog of streams exists
	query := "SELECT table_name FROM information_schema.tables WHERE table_schema = '" + s.schemaName + "' AND table_name = '" + s.catalogTableName + "'"
	row := s.pool.QueryRow(query)

	if err = row.Scan(&name); err != nil {
		if err == sql.ErrNoRows {
			// SQL table does not exist
			return false, nil
		}

		s.logger.Fatal(
			"Can't check if table exists",
			zap.String("topic", "stream"),
			zap.String("method", "CatalogExists"),
			zap.String("table", s.catalogTableName),
			zap.Error(err),
		)
		return false, err
	}

	// SQL table exists
	return true, nil
}

func (s *StreamCatalogMySQL) CreateEmptyCatalog() error {
	// create new SQL table that will hold the catalog of streams
	query := `CREATE TABLE ` + s.schemaName + `.` + s.catalogTableName + ` (
		id CHAR(36) PRIMARY KEY,
		creation_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		last_update TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		cache_cpt_rows BIGINT DEFAULT 0,
		cache_size_in_bytes BIGINT DEFAULT 0,
		cache_first_msg_id BIGINT NULL,
		cache_last_msg_id BIGINT NULL,
		cache_first_msg_timestamp TIMESTAMP NULL,
		cache_last_msg_timestamp TIMESTAMP NULL,
		comment VARCHAR(255) DEFAULT NULL,
		properties JSON DEFAULT NULL
	)`
	_, err := s.pool.Exec(query)
	if err != nil {
		s.logger.Fatal(
			"Can't create table",
			zap.String("topic", "stream"),
			zap.String("method", "CreateEmptyCatalog"),
			zap.String("table", s.catalogTableName),
			zap.Error(err),
		)
		return err
	}

	// empty the streams list
	s.streams = make(types.StreamInfoDict)
	return nil
}

func (s *StreamCatalogMySQL) SaveStreamCatalog() error {
	// nothing to do (catalog is persistent in the SQL table)
	return nil
}

func (s *StreamCatalogMySQL) GetStreamsUUIDs() types.StreamUUIDList {
	s.mu.Lock()
	var streamsUUIDs types.StreamUUIDList = make(types.StreamUUIDList, 0)
	for streamUUID := range s.streams {
		streamsUUIDs = append(streamsUUIDs, streamUUID)
	}
	s.mu.Unlock()
	return streamsUUIDs
}

func (s *StreamCatalogMySQL) StreamExists(streamUUID types.StreamUUID) bool {
	// Check if stream exists in the catalog
	// use in-memory list of streams UUIDs for performance optimization insead of querying the SQL table
	s.mu.Lock()
	defer s.mu.Unlock()

	// search for the stream UUID in the dict of streams UUIDs
	if _, found := s.streams[streamUUID]; found {
		return true
	}

	return false
}

func (s *StreamCatalogMySQL) LoadStreamCatalog() (types.StreamUUIDList, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Info(
		"Loading streams",
		zap.String("topic", "stream"),
		zap.String("method", "LoadStreamCatalog"),
	)

	// load the catalog of streams from the SQL table
	query := "SELECT id, creation_date, cache_cpt_rows, cache_size_in_bytes, cache_first_msg_id, cache_last_msg_id, cache_first_msg_timestamp, cache_last_msg_timestamp, last_update, properties FROM " + s.schemaName + "." + s.catalogTableName
	rows, err := s.pool.Query(query)
	if err != nil {
		s.logger.Fatal(
			"Can't load streams",
			zap.String("topic", "stream"),
			zap.String("method", "LoadStreamCatalog"),
			zap.String("schema", s.schemaName),
			zap.String("table", s.catalogTableName),
			zap.Error(err),
		)
		return nil, err
	}
	defer rows.Close()

	// empty the streams list
	s.streams = make(types.StreamInfoDict)

	// read the rows of the SQL table into the catalog of streams
	var streamsUUIDs types.StreamUUIDList = make(types.StreamUUIDList, 0)
	var strProperties string
	var firstMsgId sql.NullInt64
	var lastMsgId sql.NullInt64
	var firstMsgTimestamp sql.NullTime
	var lastMsgTimestamp sql.NullTime

	for rows.Next() {
		info := types.StreamInfo{}
		if err := rows.Scan(
			&info.UUID,
			&info.CreationDate,
			&info.IngestedMessages.CptMessages,
			&info.IngestedMessages.SizeInBytes,
			&firstMsgId,
			&lastMsgId,
			&firstMsgTimestamp,
			&lastMsgTimestamp,
			&info.LastUpdate,
			&strProperties,
		); err != nil {
			s.logger.Fatal(
				"Can't read stream",
				zap.String("topic", "stream"),
				zap.String("method", "LoadStreamCatalog"),
				zap.String("schema", s.schemaName),
				zap.String("table", s.catalogTableName),
				zap.Error(err),
			)
			return nil, err
		}

		if firstMsgId.Valid {
			info.IngestedMessages.FirstMsgId = uint64(firstMsgId.Int64)
		} else {
			info.IngestedMessages.FirstMsgId = 0
		}

		if lastMsgId.Valid {
			info.IngestedMessages.LastMsgId = uint64(lastMsgId.Int64)
		} else {
			info.IngestedMessages.LastMsgId = 0
		}

		if firstMsgTimestamp.Valid {
			info.IngestedMessages.FirstMsgTimestamp = firstMsgTimestamp.Time
		} else {
			info.IngestedMessages.FirstMsgTimestamp = time.Time{}
		}

		if lastMsgTimestamp.Valid {
			info.IngestedMessages.LastMsgTimestamp = lastMsgTimestamp.Time
		} else {
			info.IngestedMessages.LastMsgTimestamp = time.Time{}
		}

		info.ReadableMessages.CptMessages = info.IngestedMessages.CptMessages
		info.ReadableMessages.FirstMsgId = info.IngestedMessages.FirstMsgId
		info.ReadableMessages.LastMsgId = info.IngestedMessages.LastMsgId
		info.ReadableMessages.FirstMsgTimestamp = info.IngestedMessages.FirstMsgTimestamp
		info.ReadableMessages.LastMsgTimestamp = info.IngestedMessages.LastMsgTimestamp
		info.ReadableMessages.SizeInBytes = info.IngestedMessages.SizeInBytes

		// unmarshal properties from JSON
		info.Properties = make(types.StreamProperties)
		if err := json.Unmarshal([]byte(strProperties), &info.Properties); err != nil {
			s.logger.Fatal(
				"Can't unmarshal properties from JSON",
				zap.String("topic", "stream"),
				zap.String("method", "LoadStreamCatalog"),
				zap.String("schema", s.schemaName),
				zap.String("table", s.catalogTableName),
				zap.String("stream.uuid", info.UUID.String()),
				zap.Error(err),
			)
			return nil, err
		}

		s.streams[info.UUID] = &info
		streamsUUIDs = append(streamsUUIDs, info.UUID)
	}

	return streamsUUIDs, nil // note: cannot call s.GetStreamsUUIDs() instead due to defered mutex unlock
}

func (s *StreamCatalogMySQL) OnCreateStream(streamInfo *types.StreamInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// begin SQL transaction
	transaction, err := s.pool.Begin()
	if err != nil {
		s.logger.Error(
			"can't start transaction",
			zap.String("topic", "stream"),
			zap.String("method", "OnCreateStream"),
			zap.String("stream.uuid", streamInfo.UUID.String()),
			zap.Error(err),
		)
		return err
	}

	// insert new stream into the catalog (in catalog SQL table)
	query := "INSERT INTO " + s.schemaName + "." + s.catalogTableName + " (id, creation_date, last_update, properties) VALUES (?, ?, ?, ?)"
	propertiesJSON, err := json.Marshal(streamInfo.Properties)
	if err != nil {
		s.logger.Error(
			"Can't marshal properties to JSON",
			zap.String("topic", "stream"),
			zap.String("method", "OnCreateStream"),
			zap.String("schema", s.schemaName),
			zap.String("table", s.catalogTableName),
			zap.String("stream.uuid", streamInfo.UUID.String()),
			zap.Error(err),
		)
		return err
	}
	_, err = transaction.Exec(
		query,
		streamInfo.UUID,
		streamInfo.CreationDate.Format(time.RFC3339),
		streamInfo.LastUpdate.Format(time.RFC3339),
		propertiesJSON,
	)
	if err != nil {
		s.logger.Error(
			"Can't insert stream",
			zap.String("topic", "stream"),
			zap.String("method", "OnCreateStream"),
			zap.String("schema", s.schemaName),
			zap.String("table", s.catalogTableName),
			zap.String("stream.uuid", streamInfo.UUID.String()),
			zap.Error(err),
		)
		// rollback the transaction
		_ = transaction.Rollback()
		return err
	}

	// create the stream SQL table
	streamTableName := s.GetSQLStreamTable(streamInfo.UUID)
	query = "CREATE TABLE " + s.schemaName + "." + streamTableName + " (id BIGINT PRIMARY KEY, timestamp TIMESTAMP, message JSON)"
	_, err = transaction.Exec(query)
	if err != nil {
		s.logger.Error(
			"Can't create stream table",
			zap.String("topic", "stream"),
			zap.String("method", "OnCreateStream"),
			zap.String("schema", s.schemaName),
			zap.String("table", streamTableName),
			zap.String("stream.uuid", streamInfo.UUID.String()),
			zap.Error(err),
		)
		// rollback the transaction
		_ = transaction.Rollback()
		return err
	}

	// commit the transaction
	err = transaction.Commit()
	if err != nil {
		s.logger.Error(
			"can't commit transaction",
			zap.String("topic", "stream"),
			zap.String("method", "OnCreateStream"),
			zap.String("stream.uuid", streamInfo.UUID.String()),
			zap.Error(err),
		)
		// rollback the transaction
		_ = transaction.Rollback()
		return err
	}

	// Add stream to the catalog
	s.streams[streamInfo.UUID] = streamInfo

	return nil
}

func (s *StreamCatalogMySQL) OnDeleteStream(streamUUID types.StreamUUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove stream from the catalog (in memory dict of streams)
	delete(s.streams, streamUUID)

	// Remove stream from the catalog (in catalog SQL table)
	query := "DELETE FROM " + s.schemaName + "." + s.catalogTableName + " WHERE id = ?"
	_, err := s.pool.Exec(query, streamUUID)
	if err != nil {
		s.logger.Fatal(
			"Can't delete stream",
			zap.String("topic", "stream"),
			zap.String("method", "OnDeleteStream"),
			zap.String("schema", s.schemaName),
			zap.String("table", s.catalogTableName),
			zap.String("stream.uuid", streamUUID.String()),
			zap.Error(err),
		)
		return err
	}

	// Remove the stream SQL table
	streamTableName := s.streamTablePrefix + streamUUID.String()
	query = "DROP TABLE " + s.schemaName + "." + streamTableName
	_, err = s.pool.Exec(query)
	if err != nil {
		s.logger.Fatal(
			"Can't drop stream table",
			zap.String("topic", "stream"),
			zap.String("method", "OnDeleteStream"),
			zap.String("schema", s.schemaName),
			zap.String("table", streamTableName),
			zap.String("stream.uuid", streamUUID.String()),
			zap.Error(err),
		)
		return err
	}

	return nil
}

func (s *StreamCatalogMySQL) GetStreamInfo(streamUUID types.StreamUUID) (*types.StreamInfo, error) {
	if info, ok := s.streams[streamUUID]; ok {
		return info, nil
	}

	// stream not found (return an error)
	return nil, fmt.Errorf("stream not found %v", streamUUID)
}

func (s *StreamCatalogMySQL) GetSQLStreamTable(streamUUID types.StreamUUID) string {
	streamTableName := s.streamTablePrefix + strings.Replace(streamUUID.String(), "-", "_", -1)
	return streamTableName
}

func NewStreamCatalogMySQL(logger *zap.Logger, pool *sql.DB, schemaName string, catalogTableName string, sStreamTablePrefix string) *StreamCatalogMySQL {
	return &StreamCatalogMySQL{logger: logger, pool: pool, schemaName: schemaName, catalogTableName: catalogTableName, streamTablePrefix: sStreamTablePrefix}
}
