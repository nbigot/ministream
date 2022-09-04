package dynamodbprovider

import (
	"ministream/buffering"
	"ministream/config"
	"ministream/storageprovider"
	"ministream/types"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type DynamoDBStorage struct {
	// implements IStorageProvider interface
	logger *zap.Logger
}

func (s *DynamoDBStorage) Init() error {
	return nil
}

func (s *DynamoDBStorage) GenerateNewStreamUuid() types.StreamUUID {
	// ensure uuid is unique
	for {
		candidate := uuid.New()
		if !s.StreamExists(candidate) {
			return candidate
		}
	}
}

func (s *DynamoDBStorage) StreamExists(streamUUID types.StreamUUID) bool {
	panic("not implemented")
}

func (s *DynamoDBStorage) LoadStreams() (types.StreamInfoList, error) {
	panic("not implemented")
}

func (s *DynamoDBStorage) SaveStreamCatalog(streamUUIDs types.StreamUUIDList) error {
	panic("not implemented")
}

func (s *DynamoDBStorage) OnCreateStream(info *types.StreamInfo) error {
	panic("not implemented")
}

func (s *DynamoDBStorage) LoadStreamFromUUID(streamUUID types.StreamUUID) (*types.StreamInfo, error) {
	panic("not implemented")
}

func (s *DynamoDBStorage) SaveStream(ingestBuffer *buffering.StreamIngestBuffer, info *types.StreamInfo) error {
	panic("not implemented")
}

func (s *DynamoDBStorage) BuildIndex(streamUUID types.StreamUUID) (interface{}, error) {
	panic("not implemented")
}

func (s *DynamoDBStorage) NewStreamIteratorHandler(streamUUID types.StreamUUID, iteratorUUID types.StreamIteratorUUID) (types.IStreamIteratorHandler, error) {
	panic("not implemented")
}

func (s *DynamoDBStorage) NewStreamWriter(*types.StreamInfo) (buffering.IStreamWriter, error) {
	panic("not implemented")
}

func NewStorageProvider(logger *zap.Logger, conf *config.Config) (storageprovider.IStorageProvider, error) {
	return &DynamoDBStorage{logger: logger}, nil
}

func init() {
	err := storageprovider.Register("DynamoDB", NewStorageProvider)
	if err != nil {
		panic(err)
	}
}
