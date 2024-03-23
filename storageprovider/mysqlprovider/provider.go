package mysqlprovider

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/nbigot/ministream/buffering"
	"github.com/nbigot/ministream/config"
	"github.com/nbigot/ministream/storageprovider"
	"github.com/nbigot/ministream/storageprovider/catalog"
	"github.com/nbigot/ministream/types"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

	"go.uber.org/zap"
)

type MySQLStorage struct {
	// implements IStorageProvider interface
	logger       *zap.Logger
	logVerbosity int
	mysqlConfig  *MySQLConfig
	pool         *sql.DB
	mu           sync.Mutex
	catalog      catalog.IStorageCatalog
	indexes      map[types.StreamUUID]*StreamIndexMySQL
}

func (s *MySQLStorage) Init() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var err error
	var dsn string

	if dsn, err = SafeOverrideDSN(s.mysqlConfig.Dsn); err != nil {
		return err
	}

	s.ClearIndexes()
	s.pool, err = sql.Open("mysql", dsn)
	if err != nil {
		return err
	}

	s.pool.SetConnMaxLifetime(time.Duration(s.mysqlConfig.ConnMaxLifetime) * time.Second)
	s.pool.SetMaxIdleConns(int(s.mysqlConfig.MaxIdleConns))
	s.pool.SetMaxOpenConns(int(s.mysqlConfig.MaxOpenConns))

	// check connection
	if err = s.pool.Ping(); err != nil {
		return err
	}

	s.catalog = NewStreamCatalogMySQL(s.logger, s.pool, s.mysqlConfig.SchemaName, s.mysqlConfig.CatalogTableName, s.mysqlConfig.StreamTablePrefix)

	if err = s.catalog.Init(); err != nil {
		return err
	}

	return nil
}

func (s *MySQLStorage) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.catalog.Stop(); err != nil {
		return err
	}

	if s.pool != nil {
		return s.pool.Close()
	}

	return nil
}

func (s *MySQLStorage) GenerateNewStreamUuid() types.StreamUUID {
	// ensure new stream uuid is unique
	for {
		candidate := uuid.New()
		if !s.StreamExists(candidate) {
			return candidate
		}
	}
}

func (s *MySQLStorage) StreamExists(streamUUID types.StreamUUID) bool {
	return s.catalog.StreamExists(streamUUID)
}

func (s *MySQLStorage) LoadStreams() (types.StreamInfoList, error) {
	streamsUUID, err := s.catalog.LoadStreamCatalog()
	if err != nil {
		return types.StreamInfoList{}, err
	}

	if len(streamsUUID) > 0 {
		s.logger.Info(
			"Found streams",
			zap.String("topic", "stream"),
			zap.String("method", "LoadStreams"),
			zap.Int("streams", len(streamsUUID)),
		)
	} else {
		s.logger.Info(
			"No stream found",
			zap.String("topic", "stream"),
			zap.String("method", "LoadStreams"),
		)
	}

	var l types.StreamInfoList
	if l, err = s.LoadStreamsFromUUIDs(streamsUUID); err != nil {
		return l, err
	}

	return l, nil
}

func (s *MySQLStorage) SaveStreamCatalog() error {
	return s.catalog.SaveStreamCatalog()
}

func (s *MySQLStorage) OnCreateStream(info *types.StreamInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.catalog.OnCreateStream(info); err != nil {
		return err
	}

	// instanciate the index
	s.SetStreamIndex(info.UUID, NewStreamIndex(info.UUID, info, s.mysqlConfig.SchemaName, s.getStreamTableName(info.UUID), s.pool, s.logger))

	return nil
}

func (s *MySQLStorage) LoadStreamsFromUUIDs(streamUUIDs types.StreamUUIDList) (types.StreamInfoList, error) {
	infos := make(types.StreamInfoList, len(streamUUIDs))
	for idx, streamUUID := range streamUUIDs {
		if info, err := s.GetStreamInfo(streamUUID); err != nil {
			return nil, err
		} else {
			infos[idx] = info
			// instanciate the index
			s.SetStreamIndex(streamUUID, NewStreamIndex(streamUUID, info, s.mysqlConfig.SchemaName, s.getStreamTableName(streamUUID), s.pool, s.logger))
		}
	}
	return infos, nil
}

func (s *MySQLStorage) GetStreamInfo(streamUUID types.StreamUUID) (*types.StreamInfo, error) {
	return s.catalog.GetStreamInfo(streamUUID)
}

func (s *MySQLStorage) NewStreamIteratorHandler(streamUUID types.StreamUUID, iteratorUUID types.StreamIteratorUUID) (types.IStreamIteratorHandler, error) {
	if idx, err := s.GetStreamIndex(streamUUID); err != nil {
		return nil, err
	} else {
		return NewStreamIteratorHandlerMySQL(streamUUID, iteratorUUID, idx, s.mysqlConfig.SchemaName, s.getStreamTableName(streamUUID), s.pool, 100, s.logger), nil
	}
}

func (s *MySQLStorage) DeleteStream(streamUUID types.StreamUUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// delete index in memory
	delete(s.indexes, streamUUID)

	// delete index in catalog
	return s.catalog.OnDeleteStream(streamUUID)
}

func (s *MySQLStorage) NewStreamWriter(info *types.StreamInfo) (buffering.IStreamWriter, error) {
	w := NewStreamWriterMySQL(info, s.mysqlConfig.SchemaName, s.mysqlConfig.CatalogTableName, s.getStreamTableName(info.UUID), s.pool, s.logger, s.logVerbosity)
	return w, nil
}

func (s *MySQLStorage) BuildIndex(streamUUID types.StreamUUID) (interface{}, error) {
	if idx, err := s.GetStreamIndex(streamUUID); err != nil {
		return nil, err
	} else {
		return idx.BuildIndex()
	}
}

func (s *MySQLStorage) GetDSN() string {
	return s.mysqlConfig.Dsn
}

func (s *MySQLStorage) getStreamTableName(streamUUID types.StreamUUID) string {
	return s.mysqlConfig.StreamTablePrefix + strings.Replace(streamUUID.String(), "-", "_", -1)
}

func (s *MySQLStorage) GetStreamIndex(streamUUID types.StreamUUID) (*StreamIndexMySQL, error) {
	if idx, ok := s.indexes[streamUUID]; ok {
		return idx, nil
	}

	return nil, fmt.Errorf("stream index not found %v", streamUUID)
}

func (s *MySQLStorage) SetStreamIndex(streamUUID types.StreamUUID, idx *StreamIndexMySQL) {
	s.indexes[streamUUID] = idx
}

func (s *MySQLStorage) ClearIndexes() {
	s.indexes = make(map[types.StreamUUID]*StreamIndexMySQL)
}

func SafeOverrideDSN(dsn string) (string, error) {
	// force parseTime=true in DSN if not already set in order to parse time.Time
	// Parse the original DSN
	dsnURL, err := url.Parse(dsn)
	if err != nil {
		// Failed to parse DSN
		return dsn, err
	}

	// Parse the query part of the DSN to get existing parameters
	queryParams := dsnURL.Query()

	// Add the parseTime=true parameter
	queryParams.Set("parseTime", "true")

	// Encode the updated query parameters back to the DSN
	dsnURL.RawQuery = queryParams.Encode()

	// The updated DSN
	updatedDSN := dsnURL.String()
	return updatedDSN, nil
}

func NewStorageProvider(logger *zap.Logger, conf *config.Config) (storageprovider.IStorageProvider, error) {
	mySQLConfig, err := NewMySQLConfig(conf)
	if err != nil {
		return nil, err
	}

	return &MySQLStorage{
		logger:       logger,
		logVerbosity: conf.Storage.LogVerbosity,
		mysqlConfig:  mySQLConfig,
		catalog:      nil,
		pool:         nil,
		indexes:      make(map[types.StreamUUID]*StreamIndexMySQL),
	}, nil
}
