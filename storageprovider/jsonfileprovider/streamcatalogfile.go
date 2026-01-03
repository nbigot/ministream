package jsonfileprovider

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/nbigot/ministream/types"

	"github.com/goccy/go-json"
	"go.uber.org/zap"
)

type StreamCatalogFile struct {
	// implements IStorageCatalog interface
	logger                *zap.Logger
	dataDirectory         string // root directory to store all data and streams
	streamCatalogFilepath string // path to the file storing the catalog of streams
	mu                    sync.Mutex
	streams               types.StreamInfoDict // map of streams indexed by UUID
}

func (s *StreamCatalogFile) Init() error {
	// empty the streams list
	s.streams = make(types.StreamInfoDict)
	return s.EnsureCatalogFileExists()
}

func (s *StreamCatalogFile) Stop() error {
	return nil
}

func (s *StreamCatalogFile) EnsureCatalogFileExists() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.CatalogFileExists() {
		// file already exists therefore do nothing
		return nil
	}

	// create file having an empty catalog of streams
	return s.CreateEmptyCatalogFile()
}

func (s *StreamCatalogFile) CatalogFileExists() bool {
	if _, err := os.Stat(s.streamCatalogFilepath); err == nil {
		return true
	}
	return false
}

func (s *StreamCatalogFile) CreateEmptyCatalogFile() error {
	// create file having an empty catalog of streams
	obj := streamListSerializeStruct{}
	if sJson, err := json.Marshal(obj); err != nil {
		s.logger.Fatal(
			"Can't serialize json stream list",
			zap.String("topic", "stream"),
			zap.String("method", "EnsureCatalogFileExists"),
			zap.String("filename", s.streamCatalogFilepath),
			zap.Error(err),
		)
		return err
	} else {
		if err := os.WriteFile(s.streamCatalogFilepath, sJson, 0644); err != nil {
			s.logger.Fatal(
				"Can't save streams",
				zap.String("topic", "stream"),
				zap.String("method", "EnsureCatalogFileExists"),
				zap.String("filename", s.streamCatalogFilepath),
				zap.Error(err),
			)
			return err
		}
	}

	return nil
}

func (s *StreamCatalogFile) SaveStreamCatalog() error {
	s.logger.Info(
		"Save streams",
		zap.String("topic", "stream"),
		zap.String("method", "SaveStreamCatalog"),
		zap.String("filename", s.streamCatalogFilepath),
	)

	obj := streamListSerializeStruct{
		StreamsUUID: s.GetStreamsUUIDs(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if sJson, err := json.Marshal(obj); err != nil {
		s.logger.Fatal(
			"Can't serialize json stream list",
			zap.String("topic", "stream"),
			zap.String("method", "SaveStreamCatalog"),
			zap.String("filename", s.streamCatalogFilepath),
			zap.Error(err),
		)
		return err
	} else {
		if err := os.WriteFile(s.streamCatalogFilepath, sJson, 0644); err != nil {
			s.logger.Fatal(
				"Can't save streams",
				zap.String("topic", "stream"),
				zap.String("method", "SaveStreamCatalog"),
				zap.String("filename", s.streamCatalogFilepath),
				zap.Error(err),
			)
			return err
		}
	}

	return nil
}

func (s *StreamCatalogFile) LoadStreamCatalog() (types.StreamUUIDList, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Info(
		"Loading streams",
		zap.String("topic", "stream"),
		zap.String("method", "LoadStreamCatalog"),
		zap.String("filename", s.streamCatalogFilepath),
	)

	file, err := os.Open(s.streamCatalogFilepath)
	if err != nil {
		s.logger.Fatal(
			"Can't open stream catalog",
			zap.String("topic", "stream"),
			zap.String("method", "LoadStreamCatalog"),
			zap.String("filename", s.streamCatalogFilepath),
			zap.Error(err),
		)
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	// load the list of streams UUIDs
	obj := streamListSerializeStruct{}
	jsonDecoder := json.NewDecoder(file)
	err = jsonDecoder.Decode(&obj)
	if err != nil {
		s.logger.Fatal(
			"Can't decode json stream list",
			zap.String("topic", "stream"),
			zap.String("method", "LoadStreamCatalog"),
			zap.String("filename", s.streamCatalogFilepath),
			zap.Error(err),
		)
		return nil, err
	}

	// load all streams info from the list of streams UUIDs
	s.streams = make(types.StreamInfoDict)
	for _, streamUUID := range obj.StreamsUUID {
		info, err := s.LoadStreamFromUUID(streamUUID)
		if err != nil {
			s.logger.Error("Can't load stream",
				zap.String("topic", "stream"),
				zap.String("method", "LoadStreamCatalog"),
				zap.String("stream.uuid", streamUUID.String()), zap.Error(err),
			)
			return nil, err
		}
		s.streams[streamUUID] = info
	}

	return obj.StreamsUUID, nil
}

func (s *StreamCatalogFile) GetStreamsUUIDs() types.StreamUUIDList {
	s.mu.Lock()
	var streamsUUIDs = make(types.StreamUUIDList, 0)
	for streamUUID := range s.streams {
		streamsUUIDs = append(streamsUUIDs, streamUUID)
	}
	s.mu.Unlock()
	return streamsUUIDs
}

func (s *StreamCatalogFile) GetStreamInfo(streamUUID types.StreamUUID) (*types.StreamInfo, error) {
	if info, found := s.streams[streamUUID]; found {
		return info, nil
	}

	return nil, fmt.Errorf("stream not found %v", streamUUID)
}

func (s *StreamCatalogFile) StreamExists(streamUUID types.StreamUUID) bool {
	// Check if stream exists in the catalog
	if _, found := s.streams[streamUUID]; found {
		return true
	}

	return false
}

func (s *StreamCatalogFile) OnCreateStream(streamInfo *types.StreamInfo) error {
	// Add stream to the catalog
	s.streams[streamInfo.UUID] = streamInfo
	return nil
}

func (s *StreamCatalogFile) OnDeleteStream(streamUUID types.StreamUUID) error {
	// Remove stream from the catalog
	delete(s.streams, streamUUID)
	return nil
}

func (s *StreamCatalogFile) LoadStreamFromUUID(streamUUID types.StreamUUID) (*types.StreamInfo, error) {
	s.logger.Info(
		"Loading stream",
		zap.String("topic", "stream"),
		zap.String("method", "LoadStreamFromUUID"),
		zap.String("stream.uuid", streamUUID.String()),
	)
	info := types.StreamInfo{}

	var filename = s.GetMetaDataFilePath(streamUUID)
	file, err := os.Open(filename)
	if err != nil {
		s.logger.Error("Can't open stream",
			zap.String("topic", "stream"),
			zap.String("method", "LoadStreamFromUUID"),
			zap.String("filename", filename), zap.Error(err),
		)
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	jsonDecoder := json.NewDecoder(file)
	err = jsonDecoder.Decode(&info)
	if err != nil {
		s.logger.Error("Can't decode json stream",
			zap.String("topic", "stream"),
			zap.String("method", "LoadStreamFromUUID"),
			zap.String("filename", filename), zap.Error(err),
		)
		return nil, err
	}

	return &info, nil
}

func (s *StreamCatalogFile) GetMetaDataFilePath(streamUUID types.StreamUUID) string {
	return filepath.Join(s.GetStreamDirectoryPath(streamUUID), "stream.json")
}

func (s *StreamCatalogFile) GetStreamDirectoryPath(streamUUID types.StreamUUID) string {
	return filepath.Join(s.GetStreamsDirectoryPath(), streamUUID.String())
}

func (s *StreamCatalogFile) GetStreamsDirectoryPath() string {
	return filepath.Join(s.dataDirectory, "streams")
}

func (s *StreamCatalogFile) GetDataDirectory() string {
	return s.dataDirectory
}

func GetStreamCatalogFilepath(dataDirectory string) string {
	// streams.json stores the catalog of streams
	return filepath.Join(dataDirectory, "streams.json")
}

func NewStreamCatalogFile(logger *zap.Logger, dataDirectory string, filepath string) *StreamCatalogFile {
	return &StreamCatalogFile{logger: logger, dataDirectory: dataDirectory, streamCatalogFilepath: filepath}
}
