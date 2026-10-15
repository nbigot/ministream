package jsonfileprovider

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/nbigot/ministream/types"

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
	dataOffset       int64 // size of the data file (offset of the next record)
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
		defer func() {
			_ = fileData.Close()
		}()
	}

	// ensure file fileIndexPath exists
	if fileIndex, err2 := os.OpenFile(w.fileIndexPath, os.O_RDONLY|os.O_CREATE, 0644); err2 != nil {
		return err2
	} else {
		defer func() {
			_ = fileIndex.Close()
		}()
	}

	// Rebuild the index when it does not match the data file
	// (ex: index written by a previous version of the stream writer which wrote rows
	// in a format not readable by the index, or crash while writing)
	if consistent, errCheck := isIndexConsistent(w.fileDataPath, w.fileIndexPath); errCheck != nil {
		return errCheck
	} else if !consistent {
		w.logger.Warn(
			"Stream index is not consistent with the data file, rebuilding it",
			zap.String("topic", "stream"),
			zap.String("method", "Init"),
			zap.String("stream.uuid", w.info.UUID.String()),
			zap.String("filename", w.fileIndexPath),
		)
		idx := NewStreamIndex(w.info.UUID, w.fileIndexPath, w.logger)
		if _, errBuild := idx.BuildIndex(w.fileDataPath); errBuild != nil {
			// the stream can still be written and read sequentially
			w.logger.Error(
				"Cannot rebuild stream index",
				zap.String("topic", "stream"),
				zap.String("method", "Init"),
				zap.String("stream.uuid", w.info.UUID.String()),
				zap.Error(errBuild),
			)
		}
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

	// the offset of the next record is the size of the data file
	dataInfo, err := w.fileData.Stat()
	if err != nil {
		return err
	}
	w.dataOffset = dataInfo.Size()

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

	if w.info.ReadableMessages.CptMessages == 0 {
		// first message ever of the stream
		w.info.ReadableMessages.FirstMsgId = w.info.IngestedMessages.FirstMsgId
		w.info.ReadableMessages.LastMsgId = 0
		w.info.ReadableMessages.FirstMsgTimestamp = (*records)[0].CreationDate
	}

	// process all records of the ingest buffer
	for _, record := range *records {
		if w.logVerbosity > 1 {
			w.logger.Debug(
				"write record into file",
				zap.String("topic", "stream"),
				zap.String("method", "Write"),
				zap.String("stream.uuid", w.info.UUID.String()),
				zap.Any("msg", record),
			)
		}

		// serialize the record into a string
		bytes, err := json.Marshal(record)
		if err != nil {
			w.logger.Error(
				"json",
				zap.String("topic", "stream"),
				zap.String("method", "Write"),
				zap.String("stream.uuid", w.info.UUID.String()),
				zap.Any("msg", record),
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
		w.info.ReadableMessages.CptMessages += 1
		w.info.ReadableMessages.LastMsgTimestamp = record.CreationDate
		w.info.ReadableMessages.SizeInBytes += types.Size64(len(bytes) + 1)
		w.info.ReadableMessages.LastMsgId = record.Id

		// update the index file
		// (the rows must have the format read by StreamIndexFile: id, length, offset, timestamp)
		row := streamIndexRowMsg{
			Id:                record.Id,
			LengthInBytes:     int64(countBytesWritten),
			Offset:            w.dataOffset,
			TimestampUnixNano: record.CreationDate.UnixNano(),
		}
		w.dataOffset += int64(countBytesWritten)
		if err := binary.Write(w.fileIndex, binary.LittleEndian, row); err != nil {
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

	// note: the file must be truncated, otherwise a shorter content leaves trailing garbage
	// (the file cannot be decoded anymore and the stream cannot be loaded at startup)
	file, err := os.OpenFile(w.fileMetaInfoPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
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
	defer func() {
		_ = file.Close()
	}()

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
