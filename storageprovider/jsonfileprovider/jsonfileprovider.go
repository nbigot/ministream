package jsonfileprovider

import (
	"os"
	"path/filepath"

	"github.com/nbigot/ministream/buffering"
	"github.com/nbigot/ministream/config"
	"github.com/nbigot/ministream/storageprovider"
	"github.com/nbigot/ministream/storageprovider/catalog"
	"github.com/nbigot/ministream/types"

	"github.com/google/uuid"

	"go.uber.org/zap"
)

type FileStorage struct {
	// implements IStorageProvider interface
	logger        *zap.Logger
	logVerbosity  int
	catalog       catalog.IStorageCatalog
	dataDirectory string // root directory to store all data and streams
}

type streamListSerializeStruct struct {
	StreamsUUID types.StreamUUIDList `json:"streams"`
}

type streamRowIndex struct {
	msgId      types.MessageId
	bytesCount int32
	dateTime   int64
}

func (s *FileStorage) Init() error {
	var err error

	if err = s.CreateDataDirectory(); err != nil {
		return err
	}

	if err = s.CreateStreamsDirectory(); err != nil {
		return err
	}

	if err = s.catalog.Init(); err != nil {
		return err
	}

	return nil
}

func (s *FileStorage) Stop() error {
	return s.catalog.Stop()
}

func (s *FileStorage) GenerateNewStreamUuid() types.StreamUUID {
	// ensure new stream uuid is unique
	for {
		candidate := uuid.New()
		if !s.StreamExists(candidate) {
			return candidate
		}
	}
}

func (s *FileStorage) StreamExists(streamUUID types.StreamUUID) bool {
	return s.catalog.StreamExists(streamUUID)
}

func (s *FileStorage) LoadStreams() (types.StreamInfoList, error) {
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

	return l, nil
}

func (s *FileStorage) SaveStreamCatalog() error {
	return s.catalog.SaveStreamCatalog()
}

func (s *FileStorage) OnCreateStream(info *types.StreamInfo) error {
	if err := s.CreateStreamDirectory(info.UUID); err != nil {
		return err
	}
	return s.catalog.OnCreateStream(info)
}

func (s *FileStorage) LoadStreamsFromUUIDs(streamUUIDs types.StreamUUIDList) (types.StreamInfoList, error) {
	infos := make(types.StreamInfoList, len(streamUUIDs))
	for idx, streamUUID := range streamUUIDs {
		if info, err := s.GetStreamInfo(streamUUID); err != nil {
			return nil, err
		} else {
			infos[idx] = info
		}
	}
	return infos, nil
}

func (s *FileStorage) GetStreamInfo(streamUUID types.StreamUUID) (*types.StreamInfo, error) {
	return s.catalog.GetStreamInfo(streamUUID)
}

func (s *FileStorage) GetDataDirectory() string {
	return s.dataDirectory
}

func (s *FileStorage) GetStreamsDirectoryPath() string {
	return filepath.Join(s.dataDirectory, "streams")
}

func (s *FileStorage) GetStreamDirectoryPath(streamUUID types.StreamUUID) string {
	return filepath.Join(s.GetStreamsDirectoryPath(), streamUUID.String())
}

func (s *FileStorage) GetMetaDataFilePath(streamUUID types.StreamUUID) string {
	return filepath.Join(s.GetStreamDirectoryPath(streamUUID), "stream.json")
}

func (s *FileStorage) GetStreamDataFilePath(streamUUID types.StreamUUID) string {
	return filepath.Join(s.GetStreamDirectoryPath(streamUUID), "data.jsonl")
}

func (s *FileStorage) GetStreamIndexFilePath(streamUUID types.StreamUUID) string {
	return filepath.Join(s.GetStreamDirectoryPath(streamUUID), "index.bin")
}

func (s *FileStorage) CreateDataDirectory() error {
	return os.MkdirAll(s.GetDataDirectory(), os.ModePerm)
}

func (s *FileStorage) CreateStreamsDirectory() error {
	return os.MkdirAll(s.GetStreamsDirectoryPath(), os.ModePerm)
}

func (s *FileStorage) CreateStreamDirectory(streamUUID types.StreamUUID) error {
	return os.MkdirAll(s.GetStreamDirectoryPath(streamUUID), os.ModePerm)
}

func (s *FileStorage) NewStreamIteratorHandler(streamUUID types.StreamUUID, iteratorUUID types.StreamIteratorUUID) (types.IStreamIteratorHandler, error) {
	idx := NewStreamIndex(streamUUID, s.GetStreamIndexFilePath(streamUUID), s.logger)
	return NewStreamIteratorHandlerFile(streamUUID, iteratorUUID, s.GetStreamDataFilePath(streamUUID), idx, s.logger), nil
}

func (s *FileStorage) DeleteStream(streamUUID types.StreamUUID) error {
	if err := os.RemoveAll(s.GetStreamDirectoryPath(streamUUID)); err != nil {
		return err
	}
	return s.catalog.OnDeleteStream(streamUUID)
}

func (s *FileStorage) NewStreamWriter(info *types.StreamInfo) (buffering.IStreamWriter, error) {
	fileDataPath := s.GetStreamDataFilePath(info.UUID)
	fileIndexPath := s.GetStreamIndexFilePath(info.UUID)
	fileMetaInfoPath := s.GetMetaDataFilePath(info.UUID)
	w := NewStreamWriterFile(info, fileDataPath, fileIndexPath, fileMetaInfoPath, s.logger, s.logVerbosity)
	return w, nil
}

func (s *FileStorage) BuildIndex(streamUUID types.StreamUUID) (interface{}, error) {
	idx := NewStreamIndex(streamUUID, s.GetStreamIndexFilePath(streamUUID), s.logger)
	return idx.BuildIndex(s.GetStreamDataFilePath(streamUUID))
}

func NewStorageProvider(logger *zap.Logger, conf *config.Config) (storageprovider.IStorageProvider, error) {
	return &FileStorage{
		logger:        logger,
		logVerbosity:  conf.Storage.LogVerbosity,
		dataDirectory: conf.Storage.JSONFile.DataDirectory,
		catalog:       NewStreamCatalogFile(logger, conf.Storage.JSONFile.DataDirectory, GetStreamCatalogFilepath(conf.Storage.JSONFile.DataDirectory)),
	}, nil
}
