package storageprovider

import (
	"github.com/nbigot/ministream/buffering"
	"github.com/nbigot/ministream/types"
)

type IStorageProvider interface {
	Init() error
	Stop() error
	GenerateNewStreamUuid() types.StreamUUID
	StreamExists(streamUUID types.StreamUUID) bool
	SaveStreamCatalog() error
	LoadStreams() (types.StreamInfoList, error)
	OnCreateStream(*types.StreamInfo) error
	GetStreamInfo(streamUUID types.StreamUUID) (*types.StreamInfo, error)
	BuildIndex(streamUUID types.StreamUUID) (interface{}, error)
	NewStreamIteratorHandler(streamUUID types.StreamUUID, iteratorUUID types.StreamIteratorUUID) (types.IStreamIteratorHandler, error)
	NewStreamWriter(*types.StreamInfo) (buffering.IStreamWriter, error)
	DeleteStream(streamUUID types.StreamUUID) error
}
