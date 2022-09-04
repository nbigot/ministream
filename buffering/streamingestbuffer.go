package buffering

import (
	. "ministream/types"
	"sync"
	"time"
)

type IStreamWriter interface {
	Write(record *[]DeferedStreamRecord)
}

type StreamIngestBuffer struct {
	// variables used for defered save
	bulkFlushFrequency   time.Duration // RecordMaxBufferedTime
	bulkMaxSize          int
	channelMsg           chan DeferedStreamRecord
	msgBuffer            []DeferedStreamRecord
	bufferedStateUpdates Size64
	mu                   sync.Mutex
	writer               IStreamWriter
	// variables used for handling shutdown
	//done chan struct{}
	//wg   sync.WaitGroup
}

func NewStreamIngestBuffer(bulkFlushFrequency time.Duration, bulkMaxSize int, channelBufferSize int, writer IStreamWriter) *StreamIngestBuffer {
	return &StreamIngestBuffer{
		bulkFlushFrequency:   bulkFlushFrequency,
		bulkMaxSize:          bulkMaxSize,
		msgBuffer:            make([]DeferedStreamRecord, 0, bulkMaxSize),
		bufferedStateUpdates: 0,
		channelMsg:           make(chan DeferedStreamRecord, channelBufferSize),
		writer:               writer,
		//done:                 make(chan struct{}),
		//wg:                   sync.WaitGroup{},
	}
}

func (s *StreamIngestBuffer) PutMessage(msgId MessageId, creationDate time.Time, message interface{}) {
	s.channelMsg <- DeferedStreamRecord{Id: msgId, CreationDate: creationDate, Msg: message}
}

func (s *StreamIngestBuffer) AppendMesssage(message DeferedStreamRecord) {
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

func (s *StreamIngestBuffer) GetBuffer() []DeferedStreamRecord {
	return s.msgBuffer
}

func (s *StreamIngestBuffer) GetBulkFlushFrequency() time.Duration {
	return s.bulkFlushFrequency
}

func (s *StreamIngestBuffer) GetChannelMsg() chan DeferedStreamRecord {
	return s.channelMsg
}

func (s *StreamIngestBuffer) Save() {
	s.mu.Lock()
	s.writer.Write(&s.msgBuffer)
	s.Clear()
	s.mu.Unlock()
}
