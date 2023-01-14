package jsonfileprovider

import (
	"os"
	"strings"
	"sync"

	"github.com/nbigot/ministream/types"

	"github.com/goccy/go-json"
	"go.uber.org/zap"
)

type StreamCatalogFile struct {
	// implements IStorageCatalog interface
	logger                *zap.Logger
	streamCatalogFilepath string
	mu                    sync.Mutex
}

func (s *StreamCatalogFile) Init() error {
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

func (s *StreamCatalogFile) SaveStreamCatalog(streamUUIDs types.StreamUUIDList) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Info(
		"Save streams",
		zap.String("topic", "stream"),
		zap.String("method", "SaveStreamCatalog"),
		zap.String("filename", s.streamCatalogFilepath),
	)

	obj := streamListSerializeStruct{
		StreamsUUID: streamUUIDs,
	}
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
	defer file.Close()

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

	return obj.StreamsUUID, nil
}

func GetStreamCatalogFilepath(dataDirectory string) string {
	// streams.json stores the catalog of streams
	if strings.HasSuffix(dataDirectory, "/") {
		return dataDirectory + "streams.json"
	} else {
		return dataDirectory + "/streams.json"
	}
}

func NewStreamCatalogFile(logger *zap.Logger, filepath string) *StreamCatalogFile {
	return &StreamCatalogFile{logger: logger, streamCatalogFilepath: filepath}
}
