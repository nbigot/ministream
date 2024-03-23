package catalog

import "github.com/nbigot/ministream/types"

type IStorageCatalog interface {
	Init() error
	Stop() error
	SaveStreamCatalog() error
	LoadStreamCatalog() (types.StreamUUIDList, error)
	StreamExists(streamUUID types.StreamUUID) bool
	OnCreateStream(streamInfo *types.StreamInfo) error
	OnDeleteStream(streamUUID types.StreamUUID) error
	GetStreamInfo(streamUUID types.StreamUUID) (*types.StreamInfo, error)
	GetStreamsUUIDs() types.StreamUUIDList
}
