package inmemoryprovider

import (
	"fmt"
	"sync"

	"github.com/nbigot/ministream/types"

	"go.uber.org/zap"
)

type StreamCatalogInMemory struct {
	// implements IStorageCatalog interface
	logger  *zap.Logger
	mu      sync.Mutex
	streams types.StreamInfoDict // map of streams indexed by UUID
}

func (s *StreamCatalogInMemory) Init() error {
	// empty the streams list
	s.streams = make(types.StreamInfoDict)
	return nil
}

func (s *StreamCatalogInMemory) Stop() error {
	return nil
}

func (s *StreamCatalogInMemory) SaveStreamCatalog() error {
	// Catalog is not persistent (only in memory): all streams are lost when program shuts down
	// therefore: nothing to save
	return nil
}

func (s *StreamCatalogInMemory) LoadStreamCatalog() (types.StreamUUIDList, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Catalog is not persistent (only in memory): all streams are lost when program shuts down
	// therefore: nothing to load
	s.streams = make(types.StreamInfoDict)
	result := []types.StreamUUID{}
	// return an empty list of streams
	return result, nil
}

func (s *StreamCatalogInMemory) StreamExists(streamUUID types.StreamUUID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if stream exists in the catalog
	if _, ok := s.streams[streamUUID]; ok {
		return true
	}

	return false
}

func (s *StreamCatalogInMemory) OnCreateStream(streamInfo *types.StreamInfo) error {
	s.mu.Lock()

	// Add stream to the catalog
	s.streams[streamInfo.UUID] = streamInfo

	s.mu.Unlock()
	return nil
}

func (s *StreamCatalogInMemory) OnDeleteStream(streamUUID types.StreamUUID) error {
	s.mu.Lock()

	// Remove stream from the catalog
	delete(s.streams, streamUUID)

	s.mu.Unlock()
	return nil
}

func (s *StreamCatalogInMemory) GetStreamsUUIDs() types.StreamUUIDList {
	s.mu.Lock()
	var streamsUUIDs = make(types.StreamUUIDList, 0)
	for streamUUID := range s.streams {
		streamsUUIDs = append(streamsUUIDs, streamUUID)
	}
	s.mu.Unlock()
	return streamsUUIDs
}

func (s *StreamCatalogInMemory) GetStreamInfo(streamUUID types.StreamUUID) (*types.StreamInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if info, ok := s.streams[streamUUID]; ok {
		return info, nil
	}

	// stream not found (return an error)
	return nil, fmt.Errorf("stream not found %v", streamUUID)
}

func NewStreamCatalogInMemory(logger *zap.Logger) *StreamCatalogInMemory {
	return &StreamCatalogInMemory{logger: logger}
}
