package storageprovider

import (
	"ministream/buffering"
	"ministream/config"
	"ministream/types"

	"go.uber.org/zap"
)

type IStorageProvider interface {
	Init() error
	GenerateNewStreamUuid() types.StreamUUID
	StreamExists(streamUUID types.StreamUUID) bool
	SaveStreamCatalog(streamUUIDs types.StreamUUIDList) error
	LoadStreams() (types.StreamInfoList, error)
	OnCreateStream(*types.StreamInfo) error
	LoadStreamFromUUID(streamUUID types.StreamUUID) (*types.StreamInfo, error)
	SaveStream(ingestBuffer *buffering.StreamIngestBuffer, info *types.StreamInfo) error
	BuildIndex(streamUUID types.StreamUUID) (interface{}, error)
	NewStreamIteratorHandler(streamUUID types.StreamUUID, iteratorUUID types.StreamIteratorUUID) (types.IStreamIteratorHandler, error)
	NewStreamWriter(*types.StreamInfo) (buffering.IStreamWriter, error)
}

func NewStorageProvider(logger *zap.Logger, conf *config.Config) (IStorageProvider, error) {
	if factory, err := GetFactory(conf.Storage.Type); err != nil {
		return nil, err
	} else {
		return factory(logger, conf)
	}
}
