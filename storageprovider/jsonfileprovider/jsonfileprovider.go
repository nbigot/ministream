package jsonfileprovider

import (
	"encoding/binary"
	"errors"
	"fmt"
	"ministream/buffering"
	"ministream/config"
	"ministream/storageprovider"
	"ministream/types"
	"os"
	"strings"
	"sync"

	"github.com/goccy/go-json"
	"github.com/google/uuid"

	"go.uber.org/zap"
)

type FileStorage struct {
	// implements IStorageProvider interface
	logger        *zap.Logger
	dataDirectory string // root directory to store all data and streams
	catalog       *StreamCatalogFile
	mu            sync.Mutex
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
	if err := s.CreateDataDirectory(); err != nil {
		return err
	}

	return s.CreateStreamsDirectory()
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
	filename := s.GetMetaDataFilePath(streamUUID)
	if _, err := os.Stat(filename); errors.Is(err, os.ErrNotExist) {
		return false
	}
	return true
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

func (s *FileStorage) SaveStreamCatalog(streamUUIDs types.StreamUUIDList) error {
	return s.catalog.SaveStreamCatalog(streamUUIDs)
}

func (s *FileStorage) OnCreateStream(info *types.StreamInfo) error {
	// create stream directory
	if err := s.CreateStreamDirectory(info.UUID); err != nil {
		return err
	}

	// save stream
	if err := s.SaveStream(nil, info); err != nil {
		return err
	}

	return nil
}

func (s *FileStorage) LoadStreamsFromUUIDs(streamUUIDs types.StreamUUIDList) (types.StreamInfoList, error) {
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

func (s *FileStorage) LoadStreamFromUUID(streamUUID types.StreamUUID) (*types.StreamInfo, error) {
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
	defer file.Close()

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

func (s *FileStorage) SaveStream(ingestBuffer *buffering.StreamIngestBuffer, info *types.StreamInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	streamUUID := info.UUID
	s.logger.Debug(
		"save stream",
		zap.String("topic", "stream"),
		zap.String("method", "save"),
		zap.String("stream.uuid", streamUUID.String()),
	)

	// Open stream data file
	filename := s.GetStreamDataFilePath(streamUUID)
	fileData, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		s.logger.Error(
			"can't open data file",
			zap.String("topic", "stream"),
			zap.String("method", "save"),
			zap.String("stream.uuid", streamUUID.String()),
			zap.Any("filename", filename),
			zap.Error(err),
		)
		return err
	}
	defer fileData.Close()

	// Open stream index file
	filenameIndex := s.GetStreamIndexFilePath(streamUUID)
	fileIndex, err := os.OpenFile(filenameIndex, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		s.logger.Error(
			"can't open index file",
			zap.String("topic", "stream"),
			zap.String("method", "save"),
			zap.String("stream.uuid", streamUUID.String()),
			zap.Any("filename", filenameIndex),
			zap.Error(err),
		)
		return err
	}
	defer fileIndex.Close()

	if ingestBuffer != nil {
		s.saveIngestBuffer(ingestBuffer, info, fileData, fileIndex)
	}

	// update fileinfo
	fileInfo, err := os.Stat(filename)
	if err == nil {
		info.SizeInBytes = uint64(fileInfo.Size())
	}

	s.saveFileMetaInfo(info)

	return nil
}

func (s *FileStorage) saveIngestBuffer(ingestBuffer *buffering.StreamIngestBuffer, info *types.StreamInfo, fileData *os.File, fileIndex *os.File) error {
	ingestBuffer.Lock()
	defer ingestBuffer.Unlock()

	strStreamUUID := info.UUID.String()

	// process all records of the ingest buffer
	for _, msg := range ingestBuffer.GetBuffer() {
		s.logger.Debug(
			"save msg",
			zap.String("topic", "stream"),
			zap.String("method", "save"),
			zap.String("stream.uuid", strStreamUUID),
			zap.Any("msg", msg),
		)

		// serialize the record into a string
		bytes, err := json.Marshal(msg)
		if err != nil {
			s.logger.Error(
				"json",
				zap.String("topic", "stream"),
				zap.String("method", "save"),
				zap.String("stream.uuid", strStreamUUID),
				zap.Any("msg", msg),
				zap.Error(err),
			)
			// drop the record (should never happen)
			return err
		}

		// append the record to data file
		strjson := string(bytes)
		var countBytesWritten int
		if countBytesWritten, err = fileData.WriteString(strjson + "\n"); err != nil {
			return err
		}

		// update info
		info.CptMessages += 1
		info.LastUpdate = msg.CreationDate

		// update the index file
		// row format is: (<msg id>, <msg length in bytes>, <date>)
		var data = streamRowIndex{msg.Id, int32(countBytesWritten), msg.CreationDate.Unix()}
		if err := binary.Write(fileIndex, binary.LittleEndian, data); err != nil {
			return err
		}

	}

	ingestBuffer.Clear()
	return nil
}

func (s *FileStorage) saveFileMetaInfo(info *types.StreamInfo) error {
	streamUUID := info.UUID
	s.logger.Debug(
		"saveFileMetaInfo",
		zap.String("topic", "stream"),
		zap.String("method", "saveFileMetaInfo"),
		zap.String("stream.uuid", streamUUID.String()),
	)

	var filename = s.GetMetaDataFilePath(streamUUID)
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		s.logger.Error(
			"can't open meta info file",
			zap.String("topic", "stream"),
			zap.String("method", "saveFileMetaInfo"),
			zap.String("stream.uuid", streamUUID.String()),
			zap.Any("filename", filename),
			zap.Error(err),
		)
		return err
	}
	defer file.Close()

	// serialize message into string
	bytes, err := json.Marshal(info)
	if err != nil {
		s.logger.Error(
			"json marshal",
			zap.String("topic", "stream"),
			zap.String("method", "saveFileMetaInfo"),
			zap.String("stream.uuid", streamUUID.String()),
			zap.Any("stream", info),
			zap.Error(err),
		)
		// skip saving (should never happen)
		return err
	}
	strjson := string(bytes)
	if _, err := file.WriteString(strjson); err != nil {
		return err
	}
	return nil
}

func (s *FileStorage) GetDataDirectory() string {
	return s.dataDirectory
}

func (s *FileStorage) GetStreamsDirectoryPath() string {
	return fmt.Sprintf("%sstreams", s.dataDirectory)
}

func (s *FileStorage) GetStreamDirectoryPath(streamUUID types.StreamUUID) string {
	return fmt.Sprintf("%sstreams/%s", s.dataDirectory, streamUUID.String())
}

func (s *FileStorage) GetMetaDataFilePath(streamUUID types.StreamUUID) string {
	return fmt.Sprintf("%sstreams/%s/stream.json", s.dataDirectory, streamUUID.String())
}

func (s *FileStorage) GetStreamDataFilePath(streamUUID types.StreamUUID) string {
	return fmt.Sprintf("%sstreams/%s/data.jsonl", s.dataDirectory, streamUUID.String())
}

func (s *FileStorage) GetStreamIndexFilePath(streamUUID types.StreamUUID) string {
	return fmt.Sprintf("%sstreams/%s/index.bin", s.dataDirectory, streamUUID.String())
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
	return NewStreamIteratorHandlerFile(streamUUID, iteratorUUID, s.logger), nil
}

func (s *FileStorage) NewStreamWriter(info *types.StreamInfo) (buffering.IStreamWriter, error) {
	fileDataPath := s.GetStreamDataFilePath(info.UUID)
	fileIndexPath := s.GetStreamIndexFilePath(info.UUID)
	return NewStreamWriterFile(info, fileDataPath, fileIndexPath, s.logger)
}

// func (s *Stream) GetIndex() *StreamIndexFile {
// 	if s.index == nil {
// 		s.index = NewStreamIndex(s.info.UUID, s.logger)
// 	}
// 	return s.index
// }

func (s *FileStorage) BuildIndex(streamUUID types.StreamUUID) (interface{}, error) {
	panic("not implemented")
	//var idx *StreamIndexFile TODO
	// TODO: ICI
	//return idx.BuildIndex(s.GetDataDirectory())
}

func NewStorageProvider(logger *zap.Logger, conf *config.Config) (storageprovider.IStorageProvider, error) {
	if strings.HasSuffix(conf.Storage.JSONFile.DataDirectory, "/") {
		return nil, fmt.Errorf("conf.Storage.File.DataDirectory must not end with /")
	}

	return &FileStorage{
		logger:        logger,
		dataDirectory: conf.Storage.JSONFile.DataDirectory,
		catalog:       NewStreamCatalogFile(logger, GetStreamCatalogFilepath(conf.Storage.JSONFile.DataDirectory)),
	}, nil
}

func init() {
	err := storageprovider.Register("JSONFile", NewStorageProvider)
	if err != nil {
		panic(err)
	}
}
