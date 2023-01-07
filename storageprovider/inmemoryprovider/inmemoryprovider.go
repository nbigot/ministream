package inmemoryprovider

import (
	"fmt"
	"sync"

	"github.com/nbigot/ministream/buffering"
	"github.com/nbigot/ministream/config"
	"github.com/nbigot/ministream/storageprovider"
	"github.com/nbigot/ministream/storageprovider/catalog"
	"github.com/nbigot/ministream/types"

	"github.com/dustin/go-humanize"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type InMemoryStorage struct {
	// implements IStorageProvider interface
	logger             *zap.Logger
	logVerbosity       int
	catalog            catalog.IStorageCatalog
	mu                 sync.Mutex
	inMemoryStreams    map[types.StreamUUID]*InMemoryStream
	maxRecordsByStream uint64
	maxSizeInBytes     uint64
}

func (s *InMemoryStorage) Init() error {
	var err error

	if err = s.ClearStreams(); err != nil {
		return err
	}

	if err = s.catalog.Init(); err != nil {
		return err
	}

	return nil
}

func (s *InMemoryStorage) Stop() error {
	var err error

	if err = s.catalog.Stop(); err != nil {
		return err
	}

	if err = s.ClearStreams(); err != nil {
		return err
	}

	return nil
}

func (s *InMemoryStorage) GenerateNewStreamUuid() types.StreamUUID {
	// ensure new stream uuid is unique
	for {
		candidate := uuid.New()
		if !s.StreamExists(candidate) {
			return candidate
		}
	}
}

func (s *InMemoryStorage) StreamExists(streamUUID types.StreamUUID) bool {
	_, found := s.inMemoryStreams[streamUUID]
	return found
}

func (s *InMemoryStorage) LoadStreams() (types.StreamInfoList, error) {
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

	// Catalog is not persistent (only in memory): all streams are lost when program shuts down
	// therefore: nothing to do

	return l, nil
}

func (s *InMemoryStorage) SaveStreamCatalog(streamUUIDs types.StreamUUIDList) error {
	return s.catalog.SaveStreamCatalog(streamUUIDs)
}

func (s *InMemoryStorage) OnCreateStream(info *types.StreamInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if inMemoryStream, err := NewInMemoryStream(info, s.maxRecordsByStream, s.maxSizeInBytes); err != nil {
		return err
	} else {
		s.inMemoryStreams[info.UUID] = inMemoryStream
	}

	return nil
}

func (s *InMemoryStorage) LoadStreamsFromUUIDs(streamUUIDs types.StreamUUIDList) (types.StreamInfoList, error) {
	infos := make(types.StreamInfoList, len(streamUUIDs))
	for idx, streamUUID := range streamUUIDs {
		if info, err := s.LoadStreamFromUUID(streamUUID); err != nil {
			return nil, err
		} else {
			infos[idx] = info
		}
	}
	return infos, nil
}

func (s *InMemoryStorage) LoadStreamFromUUID(streamUUID types.StreamUUID) (*types.StreamInfo, error) {
	s.logger.Info(
		"Loading stream",
		zap.String("topic", "stream"),
		zap.String("method", "LoadStreamFromUUID"),
		zap.String("stream.uuid", streamUUID.String()),
	)

	s.logger.Error("Can't load stream (persistenct storage not implemented",
		zap.String("topic", "stream"),
		zap.String("method", "LoadStreamFromUUID"),
	)

	return nil, fmt.Errorf("cannot load stream (not implemented) %v", streamUUID)
}

func (s *InMemoryStorage) ClearStreams() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.inMemoryStreams = make(map[types.StreamUUID]*InMemoryStream, 0)
	return nil
}

func (s *InMemoryStorage) NewStreamIteratorHandler(streamUUID types.StreamUUID, iteratorUUID types.StreamIteratorUUID) (types.IStreamIteratorHandler, error) {
	inMemoryStream, found := s.inMemoryStreams[streamUUID]
	if !found {
		return nil, fmt.Errorf("stream not found: %v", streamUUID)
	}

	return NewStreamIteratorHandlerInMemory(streamUUID, iteratorUUID, inMemoryStream, s.logger), nil
}

func (s *InMemoryStorage) DeleteStream(streamUUID types.StreamUUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.inMemoryStreams, streamUUID)
	return nil
}

func (s *InMemoryStorage) BuildIndex(streamUUID types.StreamUUID) (interface{}, error) {
	// there is no index for in memory storage, therefore return fake dummy index
	return "", nil
}

func (s *InMemoryStorage) NewStreamWriter(info *types.StreamInfo) (buffering.IStreamWriter, error) {
	inMemoryStream, found := s.inMemoryStreams[info.UUID]
	if !found {
		return nil, fmt.Errorf("stream not found: %v", info.UUID)
	}

	w := NewStreamWriterInMemory(info, inMemoryStream, s.logger, s.logVerbosity)
	return w, nil
}

func NewStorageProvider(logger *zap.Logger, conf *config.Config) (storageprovider.IStorageProvider, error) {
	maxSizeInBytes, err := humanize.ParseBytes(conf.Storage.InMemory.MaxSize)
	if err != nil {
		return nil, fmt.Errorf("cannot parse value for configuration storage.inmemory.maxSize: %s", err.Error())
	}

	return &InMemoryStorage{
		logger:             logger,
		logVerbosity:       conf.Storage.LogVerbosity,
		catalog:            NewStreamCatalogInMemory(logger),
		inMemoryStreams:    make(map[types.StreamUUID]*InMemoryStream, 0),
		maxRecordsByStream: conf.Storage.InMemory.MaxRecordsByStream,
		maxSizeInBytes:     maxSizeInBytes,
	}, nil
}
