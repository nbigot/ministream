package inmemoryprovider

import (
	"ministream/types"

	"go.uber.org/zap"
)

type StreamCatalogInMemory struct {
	// implements IStorageCatalog interface
	logger *zap.Logger
}

func (s *StreamCatalogInMemory) Init() error {
	return nil
}

func (s *StreamCatalogInMemory) Stop() error {
	return nil
}

func (s *StreamCatalogInMemory) SaveStreamCatalog(streamUUIDs types.StreamUUIDList) error {
	// Catalog is not persistent (only in memory): all streams are lost when program shuts down
	// therefore: nothing to save
	return nil
}

func (s *StreamCatalogInMemory) LoadStreamCatalog() (types.StreamUUIDList, error) {
	// Catalog is not persistent (only in memory): all streams are lost when program shuts down
	// therefore: nothing to load
	streamsUUID := make(types.StreamUUIDList, 0)
	return streamsUUID, nil
}

func NewStreamCatalogInMemory(logger *zap.Logger) *StreamCatalogInMemory {
	return &StreamCatalogInMemory{logger: logger}
}
