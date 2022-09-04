package jsonfileprovider

import (
	"io/ioutil"
	"ministream/types"
	"os"
	"strings"
	"sync"

	"github.com/goccy/go-json"
	"go.uber.org/zap"
)

type StreamCatalogFile struct {
	logger                *zap.Logger
	streamCatalogFilepath string
	mu                    sync.Mutex
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
		if err := ioutil.WriteFile(s.streamCatalogFilepath, sJson, 0644); err != nil {
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
