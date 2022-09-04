package mysqlprovider

import (
	"ministream/buffering"
	"ministream/config"
	"ministream/storageprovider"
	"ministream/types"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type MySQLStorage struct {
	// implements IStorageProvider interface
	logger *zap.Logger
}

func (s *MySQLStorage) Init() error {
	return nil
}

func (s *MySQLStorage) GenerateNewStreamUuid() types.StreamUUID {
	// ensure uuid is unique
	for {
		candidate := uuid.New()
		if !s.StreamExists(candidate) {
			return candidate
		}
	}
}

func (s *MySQLStorage) StreamExists(streamUUID types.StreamUUID) bool {
	panic("not implemented")
}

func (s *MySQLStorage) LoadStreams() (types.StreamInfoList, error) {
	panic("not implemented")
}

func (s *MySQLStorage) SaveStreamCatalog(streamUUIDs types.StreamUUIDList) error {
	panic("not implemented")
}

func (s *MySQLStorage) OnCreateStream(info *types.StreamInfo) error {
	panic("not implemented")
}

func (s *MySQLStorage) LoadStreamFromUUID(streamUUID types.StreamUUID) (*types.StreamInfo, error) {
	panic("not implemented")
}

func (s *MySQLStorage) SaveStream(ingestBuffer *buffering.StreamIngestBuffer, info *types.StreamInfo) error {
	panic("not implemented")
}

func (s *MySQLStorage) BuildIndex(streamUUID types.StreamUUID) (interface{}, error) {
	panic("not implemented")
}

func (s *MySQLStorage) NewStreamIteratorHandler(streamUUID types.StreamUUID, iteratorUUID types.StreamIteratorUUID) (types.IStreamIteratorHandler, error) {
	panic("not implemented")
}

func (s *MySQLStorage) NewStreamWriter(*types.StreamInfo) (buffering.IStreamWriter, error) {
	panic("not implemented")
}

func NewStorageProvider(logger *zap.Logger, conf *config.Config) (storageprovider.IStorageProvider, error) {
	return &MySQLStorage{logger: logger}, nil
}

func init() {
	err := storageprovider.Register("MySql", NewStorageProvider)
	if err != nil {
		panic(err)
	}
}
