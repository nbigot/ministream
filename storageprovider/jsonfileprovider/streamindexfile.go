package jsonfileprovider

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sync"
	"time"

	"github.com/nbigot/ministream/config"
	. "github.com/nbigot/ministream/types"

	"github.com/goccy/go-json"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type MsgOffset = int64

type StreamIndexFile struct {
	streamUUID   uuid.UUID
	logger       *zap.Logger
	logVerbosity int
	filename     string
	file         *os.File
	mu           sync.Mutex
}

type StreamIndexStats struct {
	CptMessages       int64
	FileSize          int64
	FirstMsgId        MessageId
	LastMsgId         MessageId
	FirstMsgTimestamp time.Time
	LastMsgTimestamp  time.Time
}

type streamIndexRowMsg struct {
	Id                MessageId
	LengthInBytes     int64
	Offset            int64
	TimestampUnixNano int64
}

const sizeOfStreamIndexRowMsg int64 = 4 * 8 // 4 fields x 8 bytes per field

func GetIndexFilePath(streamUUID uuid.UUID) string {
	return fmt.Sprintf("%sstreams/%s/index.bin", config.Configuration.DataDirectory, streamUUID.String())
}

func (idx *StreamIndexFile) Close() error {
	if idx.file != nil {
		return idx.file.Close()
	}

	return nil
}

func (idx *StreamIndexFile) BuildIndex(dataFilePath string) (*StreamIndexStats, error) {
	// Build or rebuild index
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.logger.Info(
		"Build index started",
		zap.String("topic", "index"),
		zap.String("method", "BuildIndex"),
		zap.String("stream.uuid", idx.streamUUID.String()),
		zap.String("index.filename", idx.filename),
	)
	stats := StreamIndexStats{CptMessages: 0, FileSize: 0}

	var err error
	if idx.file != nil {
		idx.file.Close()
	}

	var streamDataFile *os.File
	streamDataFile, err = os.OpenFile(dataFilePath, os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer streamDataFile.Close()

	idx.file, err = os.OpenFile(idx.filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		idx.logger.Error(
			"can't open index file",
			zap.String("topic", "index"),
			zap.String("method", "BuildIndex"),
			zap.String("stream.uuid", idx.streamUUID.String()),
			zap.Any("filename", idx.filename),
			zap.Error(err),
		)
		return nil, err
	}
	defer idx.file.Close()
	defer idx.file.Sync()

	var msgOffset MsgOffset = 0
	var message *DeferedStreamRecord = nil
	row := streamIndexRowMsg{}

	reader := bufio.NewReaderSize(streamDataFile, 1024*1024)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			// err is ofter io.EOF (end of file reached)
			// err may also raise when EOL char was not found
			if err == io.EOF {
				break
			} else {
				idx.logger.Error(
					"Error while building index",
					zap.String("topic", "index"),
					zap.String("method", "BuildIndex"),
					zap.String("detail", "can't read stream file"),
					zap.String("stream.uuid", idx.streamUUID.String()),
					zap.String("index.filename", idx.filename),
					zap.Int64("offset", msgOffset),
					zap.Error(err),
				)
				return nil, err
			}
		}

		err2 := json.Unmarshal([]byte(line), &message)
		if err2 != nil {
			idx.logger.Error(
				"Error while building index",
				zap.String("topic", "index"),
				zap.String("method", "BuildIndex"),
				zap.String("detail", "can't decode json message"),
				zap.String("stream.uuid", idx.streamUUID.String()),
				zap.String("index.filename", idx.filename),
				zap.Int64("offset", msgOffset),
				zap.Error(err),
			)
			return nil, err2
		}

		row.Id = message.Id
		row.LengthInBytes = int64(len(line))
		row.Offset = msgOffset
		row.TimestampUnixNano = message.CreationDate.UnixNano()

		err = binary.Write(idx.file, binary.LittleEndian, row)
		if err != nil {
			idx.logger.Error(
				"Error while writing index into file",
				zap.String("topic", "index"),
				zap.String("method", "BuildIndex"),
				zap.String("stream.uuid", idx.streamUUID.String()),
				zap.String("index.filename", idx.filename),
				zap.Int64("offset", msgOffset),
				zap.Error(err),
			)
			return nil, err
		}

		if idx.logVerbosity > 0 {
			idx.logger.Debug(
				"idx msg",
				zap.Int64("msglen", row.LengthInBytes),
				zap.Int64("offset", msgOffset),
				zap.Uint64("msgindex", message.Id),
				zap.Time("timestamp", message.CreationDate),
			)
		}

		if msgOffset == 0 {
			stats.FirstMsgId = message.Id
			stats.FirstMsgTimestamp = message.CreationDate
		}

		msgOffset += row.LengthInBytes
		stats.CptMessages += 1
	}

	if message != nil {
		stats.LastMsgId = message.Id
		stats.LastMsgTimestamp = message.CreationDate
		stats.FileSize = msgOffset
	}

	idx.logger.Info(
		"Build index ended",
		zap.String("topic", "index"),
		zap.String("method", "BuildIndex"),
		zap.String("stream.uuid", idx.streamUUID.String()),
		zap.String("index.filename", idx.filename),
		zap.Int64("index.byteSize", stats.FileSize),
		zap.Int64("index.rowsCount", stats.CptMessages),
		zap.Uint64("index.firstMsgId", stats.FirstMsgId),
		zap.Uint64("index.lastMsgId", stats.LastMsgId),
		zap.Time("index.firstMsgTimestamp", stats.FirstMsgTimestamp),
		zap.Time("index.lastMsgTimestamp", stats.LastMsgTimestamp),
	)
	return &stats, nil
}

func (idx *StreamIndexFile) GetOffsetFirstMessage() (MessageId, MsgOffset, error) {
	return 0, 0, nil
}

func (idx *StreamIndexFile) GetOffsetLastMessage() (MessageId, MsgOffset, error) {
	row, err := idx.getOffsetMessage(-sizeOfStreamIndexRowMsg, io.SeekEnd)
	if err != nil {
		return 0, 0, err
	}

	return row.Id, row.Offset, nil
}

func (idx *StreamIndexFile) GetOffsetAfterLastMessage() (MessageId, MsgOffset, error) {
	row, err := idx.getOffsetMessage(-sizeOfStreamIndexRowMsg, io.SeekEnd)
	if err != nil {
		return 0, 0, err
	}

	return row.Id + 1, row.Offset + row.LengthInBytes, nil
}

func (idx *StreamIndexFile) GetOffsetAtMessageId(messageId MessageId) (MessageId, MsgOffset, error) {
	row, err := idx.getOffsetAt(&messageId, nil)
	if err != nil {
		return 0, 0, err
	}

	return row.Id, row.Offset, nil
}

func (idx *StreamIndexFile) GetOffsetAfterMessageId(messageId MessageId) (MessageId, MsgOffset, error) {
	row, err := idx.getOffsetAt(&messageId, nil)
	if err != nil {
		return 0, 0, err
	}

	return row.Id + 1, row.Offset + row.LengthInBytes, nil
}

func (idx *StreamIndexFile) GetOffsetAtTimestamp(timestamp *time.Time) (MessageId, MsgOffset, error) {
	row, err := idx.getOffsetAt(nil, timestamp)
	if err != nil {
		return 0, 0, err
	}

	return row.Id, row.Offset, nil
}

func (idx *StreamIndexFile) getOffsetMessage(seekOffset int64, seekWhence int) (*streamIndexRowMsg, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	var err error
	if idx.file, err = os.Open(idx.filename); err != nil {
		idx.logger.Error(
			"Error while GetOffsetLastMessage open index file",
			zap.String("topic", "index"),
			zap.String("method", "getOffsetMessage"),
			zap.String("stream.uuid", idx.streamUUID.String()),
			zap.String("index.filename", idx.filename),
			zap.Error(err),
		)
		return nil, err
	}
	defer idx.file.Close()

	if _, err = idx.file.Seek(seekOffset, seekWhence); err != nil {
		idx.logger.Error(
			"Error while GetOffsetLastMessage seek",
			zap.String("topic", "index"),
			zap.String("method", "getOffsetMessage"),
			zap.String("stream.uuid", idx.streamUUID.String()),
			zap.String("index.filename", idx.filename),
			zap.Error(err),
		)
		return nil, err
	}

	row := streamIndexRowMsg{}
	if err = binary.Read(idx.file, binary.LittleEndian, &row); err != nil {
		idx.logger.Error(
			"Error while GetOffsetLastMessage read bytes",
			zap.String("topic", "index"),
			zap.String("method", "getOffsetMessage"),
			zap.String("stream.uuid", idx.streamUUID.String()),
			zap.String("index.filename", idx.filename),
			zap.Error(err),
		)
		return nil, err
	}

	return &row, nil
}

func (idx *StreamIndexFile) getOffsetAt(messageId *MessageId, timestamp *time.Time) (*streamIndexRowMsg, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	var err error
	if idx.file, err = os.Open(idx.filename); err != nil {
		idx.logger.Error(
			"Error while getOffsetAt open index file",
			zap.String("topic", "index"),
			zap.String("method", "getOffsetAt"),
			zap.String("stream.uuid", idx.streamUUID.String()),
			zap.String("index.filename", idx.filename),
			zap.Error(err),
		)
		return nil, err
	}
	defer idx.file.Close()

	var indexRowsCount int64
	if indexRowsCount, err = idx.getIndexRowsCount(); err != nil {
		return nil, err
	}
	if indexRowsCount == 0 {
		// index is empty, therefore message id cannot be found
		return nil, nil
	}
	lastIndexRank := indexRowsCount - 1

	var row streamIndexRowMsg
	if messageId != nil {
		return &row, idx.searchMessageId(*messageId, lastIndexRank, &row)
	} else {
		return &row, idx.searchTimestamp(timestamp.UnixNano(), lastIndexRank, &row)
	}
}

func (idx *StreamIndexFile) searchMessageId(messageId MessageId, lastIndexRank int64, row *streamIndexRowMsg) error {
	// use a dichotomy algorithm to find the index rank for the given MessageId
	// the result will be returned into the row variable
	// if no result found then return an error "message id not found"
	// assume every message has a unique id
	// assume message id values are always increasing as the index rank increase
	var err error
	var lowIndexRank int64 = 0
	var highIndexRank int64 = lastIndexRank
	var nextMedianIndexRank int64

	for lowIndexRank < highIndexRank {
		nextMedianIndexRank = int64(uint64(lowIndexRank+highIndexRank) >> 1)

		if err = idx.getRowAtIndexPos(nextMedianIndexRank, row); err != nil {
			return err
		}
		if row.Id == messageId {
			// message id was found
			return nil
		}
		if row.Id > messageId {
			highIndexRank = nextMedianIndexRank
		} else {
			lowIndexRank = nextMedianIndexRank + 1
		}
	}

	if err = idx.getRowAtIndexPos(lowIndexRank, row); err != nil {
		return err
	}
	if row.Id == messageId {
		// message id was found
		return nil
	}

	// can't find message id
	return errors.New("message id not found")
}

func (idx *StreamIndexFile) searchTimestamp(timestampUnixNano int64, lastIndexRank int64, row *streamIndexRowMsg) error {
	// use a dichotomy algorithm to find the index rank for the given timestamp
	// the result will be returned into the row variable
	// if no result found then return an error "message id not found"
	// assume every message has a unique id
	// assume message id values are always increasing as the index rank increase
	var err error
	var lowIndexRank int64 = 0
	var highIndexRank int64 = lastIndexRank
	var nextMedianIndexRank int64

	for lowIndexRank < highIndexRank {
		nextMedianIndexRank = int64(uint64(lowIndexRank+highIndexRank) >> 1)

		if err = idx.getRowAtIndexPos(nextMedianIndexRank, row); err != nil {
			return err
		}
		if row.TimestampUnixNano == timestampUnixNano {
			// exact timestamp was found
			return nil
		}
		if row.TimestampUnixNano > timestampUnixNano {
			highIndexRank = nextMedianIndexRank
		} else {
			lowIndexRank = nextMedianIndexRank + 1
		}
	}

	if err = idx.getRowAtIndexPos(lowIndexRank, row); err != nil {
		return err
	}
	if row.TimestampUnixNano >= timestampUnixNano {
		// found a message that was created after the given timestamp
		return nil
	}

	// can't find message created after timestamp
	return errors.New("message id not found")
}

func (idx *StreamIndexFile) getRowAtIndexPos(indexPos int64, row *streamIndexRowMsg) error {
	var err error
	if _, err = idx.file.Seek(indexPos*sizeOfStreamIndexRowMsg, io.SeekStart); err != nil {
		idx.logger.Error(
			"Error while getRowAtIndexPos seek",
			zap.String("topic", "index"),
			zap.String("method", "getRowAtIndexPos"),
			zap.String("stream.uuid", idx.streamUUID.String()),
			zap.String("index.filename", idx.filename),
			zap.Error(err),
		)
		return err
	}

	if err = binary.Read(idx.file, binary.LittleEndian, row); err != nil {
		idx.logger.Error(
			"Error while getRowAtIndexPos read bytes",
			zap.String("topic", "index"),
			zap.String("method", "getRowAtIndexPos"),
			zap.String("stream.uuid", idx.streamUUID.String()),
			zap.String("index.filename", idx.filename),
			zap.Error(err),
		)
		return err
	}

	return nil
}

func (idx *StreamIndexFile) getIndexRowsCount() (int64, error) {
	// Compute index rows count
	var err error
	var info fs.FileInfo
	if info, err = os.Stat(idx.filename); err != nil {
		return 0, err
	}

	return info.Size() / sizeOfStreamIndexRowMsg, nil
}

func (idx *StreamIndexFile) Log() {
	idx.logger.Info(
		"StreamIndex",
		zap.String("topic", "index"),
		zap.String("method", "Log"),
		zap.String("stream.uuid", idx.streamUUID.String()),
		zap.String("index.filepath", idx.filename),
	)
}

func NewStreamIndex(streamUUID uuid.UUID, logger *zap.Logger) *StreamIndexFile {
	return &StreamIndexFile{
		streamUUID:   streamUUID,
		logger:       logger,
		logVerbosity: 0,
		filename:     GetIndexFilePath(streamUUID),
		file:         nil,
	}
}
