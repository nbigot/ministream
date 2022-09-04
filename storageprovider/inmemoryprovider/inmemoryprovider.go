package inmemoryprovider

import (
	"ministream/buffering"
	"ministream/config"
	"ministream/storageprovider"
	"ministream/types"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type InMemoryStorage struct {
	// implements IStorageProvider interface
	logger *zap.Logger
}

func (s *InMemoryStorage) Init() error {
	return nil
}

func (s *InMemoryStorage) GenerateNewStreamUuid() types.StreamUUID {
	// ensure uuid is unique
	for {
		candidate := uuid.New()
		if !s.StreamExists(candidate) {
			return candidate
		}
	}
}

func (s *InMemoryStorage) StreamExists(streamUUID types.StreamUUID) bool {
	panic("not implemented")
}

func (s *InMemoryStorage) LoadStreams() (types.StreamInfoList, error) {
	// nothing to load, as it is already in memory
	panic("not implemented")
}

func (s *InMemoryStorage) SaveStreamCatalog(streamUUIDs types.StreamUUIDList) error {
	panic("not implemented")
}

func (s *InMemoryStorage) OnCreateStream(info *types.StreamInfo) error {
	panic("not implemented")
}

func (s *InMemoryStorage) LoadStreamFromUUID(streamUUID types.StreamUUID) (*types.StreamInfo, error) {
	panic("not implemented")
}

func (s *InMemoryStorage) SaveStream(ingestBuffer *buffering.StreamIngestBuffer, info *types.StreamInfo) error {
	panic("not implemented")
}

func (s *InMemoryStorage) BuildIndex(streamUUID types.StreamUUID) (interface{}, error) {
	panic("not implemented")
}

func (s *InMemoryStorage) NewStreamIteratorHandler(streamUUID types.StreamUUID, iteratorUUID types.StreamIteratorUUID) (types.IStreamIteratorHandler, error) {
	panic("not implemented")
}

func (s *InMemoryStorage) NewStreamWriter(*types.StreamInfo) (buffering.IStreamWriter, error) {
	panic("not implemented")
}

func NewStorageProvider(logger *zap.Logger, conf *config.Config) (storageprovider.IStorageProvider, error) {
	return &InMemoryStorage{logger: logger}, nil
}

func init() {
	err := storageprovider.Register("InMemory", NewStorageProvider)
	if err != nil {
		panic(err)
	}
}
