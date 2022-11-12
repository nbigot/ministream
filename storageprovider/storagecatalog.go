package storageprovider

import "ministream/types"

type IStorageCatalog interface {
	Init() error
	Stop() error
	SaveStreamCatalog(streamUUIDs types.StreamUUIDList) error
	LoadStreamCatalog() (types.StreamUUIDList, error)
}
