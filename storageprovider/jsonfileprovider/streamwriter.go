package jsonfileprovider

import (
	"encoding/binary"
	"errors"
	"fmt"
	"ministream/types"
	"os"
	"path/filepath"
	"sync"

	"github.com/goccy/go-json"
	"go.uber.org/zap"
)

const STREAM_WRITER_FILE_STATE_NONE = 0
const STREAM_WRITER_FILE_STATE_OPENED = 1
const STREAM_WRITER_FILE_STATE_CLOSED = 2

type StreamWriterFile struct {
	// implements IStreamWriter
	logger           *zap.Logger
	logVerbosity     int
	info             *types.StreamInfo
	fileDataPath     string
	fileIndexPath    string
	fileMetaInfoPath string
	fileData         *os.File
	fileIndex        *os.File
	mu               sync.Mutex
	state            int
}

func (w *StreamWriterFile) Init() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	var err error

	// ensure directory exists (or create it)
	dir := filepath.Dir(w.fileDataPath)
	if err = os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}

	// ensure file fileDataPath exists (or create it)
	if fileData, err1 := os.OpenFile(w.fileDataPath, os.O_RDONLY|os.O_CREATE, 0644); err1 != nil {
		return err1
	} else {
		defer fileData.Close()
	}

	// ensure file fileIndexPath exists
	if fileIndex, err2 := os.OpenFile(w.fileIndexPath, os.O_RDONLY|os.O_CREATE, 0644); err2 != nil {
		return err2
	} else {
		defer fileIndex.Close()
	}

	// If stream meta file does not exists then create it for the first time
	if _, errStat := os.Stat(w.fileMetaInfoPath); errors.Is(errStat, os.ErrNotExist) {
		err = w.SaveFileMetaInfo()
		if err != nil {
			return err
		}
	}

	return nil
}

func (w *StreamWriterFile) Open() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	var err error

	if w.state == STREAM_WRITER_FILE_STATE_OPENED {
		return fmt.Errorf("cannot open stream writer file because it's already opened")
	}

	// Open stream data file (os.File must stay opened for further writing)
	w.fileData, err = os.OpenFile(w.fileDataPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		w.logger.Error(
			"can't open data file",
			zap.String("topic", "stream"),
			zap.String("method", "save"),
			zap.String("stream.uuid", w.info.UUID.String()),
			zap.Any("filename", w.fileDataPath),
			zap.Error(err),
		)
		return err
	}

	// Open stream index file
	w.fileIndex, err = os.OpenFile(w.fileIndexPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		w.logger.Error(
			"can't open index file",
			zap.String("topic", "stream"),
			zap.String("method", "save"),
			zap.String("stream.uuid", w.info.UUID.String()),
			zap.Any("filename", w.fileIndexPath),
			zap.Error(err),
		)
		return err
	}

	w.state = STREAM_WRITER_FILE_STATE_OPENED
	return nil
}

func (w *StreamWriterFile) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.state != STREAM_WRITER_FILE_STATE_OPENED {
		return fmt.Errorf("cannot close stream writer file because it's not opened")
	}

	if err := w.fileData.Close(); err != nil {
		w.logger.Error(
			"can't close data file",
			zap.String("topic", "streamwriter"),
			zap.String("method", "close"),
			zap.Any("filename", w.fileData.Name()),
			zap.Error(err),
		)
		return err
	}

	if err := w.fileIndex.Close(); err != nil {
		w.logger.Error(
			"can't close index file",
			zap.String("topic", "streamwriter"),
			zap.String("method", "close"),
			zap.Any("filename", w.fileIndex.Name()),
			zap.Error(err),
		)
		return err
	}

	w.state = STREAM_WRITER_FILE_STATE_CLOSED

	w.logger.Debug(
		"Stream writer file state closed",
		zap.String("topic", "streamwriter"),
		zap.String("method", "close"),
		zap.Any("filename", w.fileIndex.Name()),
	)

	return nil
}

func (w *StreamWriterFile) Write(records *[]types.DeferedStreamRecord) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.state != STREAM_WRITER_FILE_STATE_OPENED {
		return fmt.Errorf("cannot write to stream writer file because it's not opened")
	}

	if len(*records) == 0 {
		return nil
	}

	if w.logVerbosity > 0 {
		w.logger.Debug(
			"write records into file",
			zap.String("topic", "stream"),
			zap.String("method", "Write"),
			zap.String("stream.uuid", w.info.UUID.String()),
			zap.Int("records.cpt", len(*records)),
		)
	}

	// process all records of the ingest buffer
	for _, msg := range *records {
		if w.logVerbosity > 1 {
			w.logger.Debug(
				"write record into file",
				zap.String("topic", "stream"),
				zap.String("method", "Write"),
				zap.String("stream.uuid", w.info.UUID.String()),
				zap.Any("msg", msg),
			)
		}

		// serialize the record into a string
		bytes, err := json.Marshal(msg)
		if err != nil {
			w.logger.Error(
				"json",
				zap.String("topic", "stream"),
				zap.String("method", "Write"),
				zap.String("stream.uuid", w.info.UUID.String()),
				zap.Any("msg", msg),
				zap.Error(err),
			)
			// drop the record (should never happen)
			return err
		}

		// append the record to data file
		strjson := string(bytes)
		var countBytesWritten int
		if countBytesWritten, err = w.fileData.WriteString(strjson + "\n"); err != nil {
			return err
		}

		// update info
		w.info.CptMessages += 1
		w.info.SizeInBytes += types.Size64(len(bytes) + 1)
		w.info.LastUpdate = msg.CreationDate

		// update the index file
		// row format is: (<msg id>, <msg length in bytes>, <date>)
		var data = streamRowIndex{msg.Id, int32(countBytesWritten), msg.CreationDate.Unix()}
		if err := binary.Write(w.fileIndex, binary.LittleEndian, data); err != nil {
			return err
		}
	}

	return w.SaveFileMetaInfo()
}

func (w *StreamWriterFile) SaveFileMetaInfo() error {
	streamUUID := w.info.UUID
	if w.logVerbosity > 0 {
		w.logger.Debug(
			"saveFileMetaInfo",
			zap.String("topic", "stream"),
			zap.String("method", "saveFileMetaInfo"),
			zap.String("stream.uuid", streamUUID.String()),
		)
	}

	file, err := os.OpenFile(w.fileMetaInfoPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		w.logger.Error(
			"can't open meta info file",
			zap.String("topic", "stream"),
			zap.String("method", "saveFileMetaInfo"),
			zap.String("stream.uuid", streamUUID.String()),
			zap.Any("filename", w.fileMetaInfoPath),
			zap.Error(err),
		)
		return err
	}
	defer file.Close()

	// serialize message into string
	bytes, err := json.Marshal(w.info)
	if err != nil {
		w.logger.Error(
			"json marshal",
			zap.String("topic", "stream"),
			zap.String("method", "saveFileMetaInfo"),
			zap.String("stream.uuid", streamUUID.String()),
			zap.Any("stream", w.info),
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

func NewStreamWriterFile(info *types.StreamInfo, fileDataPath string, fileIndexPath string, fileMetaInfoPath string, logger *zap.Logger, logVerbosity int) *StreamWriterFile {
	return &StreamWriterFile{
		logger:           logger,
		logVerbosity:     logVerbosity,
		info:             info,
		state:            STREAM_WRITER_FILE_STATE_NONE,
		fileDataPath:     fileDataPath,
		fileIndexPath:    fileIndexPath,
		fileMetaInfoPath: fileMetaInfoPath,
		fileData:         nil,
		fileIndex:        nil,
	}
}
