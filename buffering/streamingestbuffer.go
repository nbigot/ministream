package buffering

import (
	"sync"
	"time"

	"github.com/nbigot/ministream/types"
)

type IStreamWriter interface {
	Init() error
	Open() error
	Close() error
	Write(record *[]types.DeferedStreamRecord) error
}

type StreamIngestBuffer struct {
	// variables used for defered save
	bulkFlushFrequency   time.Duration // RecordMaxBufferedTime
	bulkMaxSize          int
	channelMsg           chan types.DeferedStreamRecord
	msgBuffer            []types.DeferedStreamRecord
	bufferedStateUpdates types.Size64
	mu                   sync.Mutex
	writer               IStreamWriter
}

func NewStreamIngestBuffer(bulkFlushFrequency time.Duration, bulkMaxSize int, channelBufferSize int, writer IStreamWriter) *StreamIngestBuffer {
	return &StreamIngestBuffer{
		bulkFlushFrequency:   bulkFlushFrequency,
		bulkMaxSize:          bulkMaxSize,
		msgBuffer:            make([]types.DeferedStreamRecord, 0, bulkMaxSize),
		bufferedStateUpdates: 0,
		channelMsg:           make(chan types.DeferedStreamRecord, channelBufferSize),
		writer:               writer,
	}
}

func (s *StreamIngestBuffer) PutMessage(msgId types.MessageId, creationDate time.Time, message interface{}) {
	s.channelMsg <- types.DeferedStreamRecord{Id: msgId, CreationDate: creationDate, Msg: message}
}

func (s *StreamIngestBuffer) AppendMesssage(message types.DeferedStreamRecord) {
	s.mu.Lock()
	s.msgBuffer = append(s.msgBuffer, message)
	s.mu.Unlock()
}

func (s *StreamIngestBuffer) IsFull() bool {
	return len(s.msgBuffer) >= s.bulkMaxSize
}

func (s *StreamIngestBuffer) Lock() {
	s.mu.Lock()
}

func (s *StreamIngestBuffer) Unlock() {
	s.mu.Unlock()
}

func (s *StreamIngestBuffer) Clear() {
	s.msgBuffer = nil
}

func (s *StreamIngestBuffer) GetBuffer() *[]types.DeferedStreamRecord {
	return &s.msgBuffer
}

func (s *StreamIngestBuffer) GetBulkFlushFrequency() time.Duration {
	return s.bulkFlushFrequency
}

func (s *StreamIngestBuffer) GetChannelMsg() chan types.DeferedStreamRecord {
	return s.channelMsg
}

func (s *StreamIngestBuffer) Save() error {
	s.Lock()
	defer s.Unlock()

	if err := s.writer.Write(&s.msgBuffer); err != nil {
		return err
	}
	s.Clear()
	return nil
}

func (s *StreamIngestBuffer) Close() error {
	s.Lock()
	defer s.Unlock()
	return s.writer.Close()
}
