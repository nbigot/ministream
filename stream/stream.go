package stream

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nbigot/ministream/buffering"
	. "github.com/nbigot/ministream/types"

	"github.com/dustin/go-humanize"
	"github.com/itchyny/gojq"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

const STREAM_STATE_NONE = 0
const STREAM_STATE_STARTING = 1
const STREAM_STATE_RUNNING = 2
const STREAM_STATE_STOPPING = 3

type Stream struct {
	info         *StreamInfo
	logger       *zap.Logger
	logVerbosity int
	iterators    StreamIteratorMap
	ingestBuffer *buffering.StreamIngestBuffer
	muIncMsgId   sync.Mutex
	done         chan struct{}
	wg           sync.WaitGroup
	state        int
}

func (s *Stream) setState(state int) {
	var strState string
	switch state {
	case STREAM_STATE_NONE:
		strState = "none"
	case STREAM_STATE_STARTING:
		strState = "starting"
	case STREAM_STATE_RUNNING:
		strState = "running"
	case STREAM_STATE_STOPPING:
		strState = "stopping"
	}

	if s.logVerbosity > 0 {
		s.logger.Debug(
			"Set stream state",
			zap.String("topic", "stream"),
			zap.String("method", "setState"),
			zap.String("stream.uuid", s.info.UUID.String()),
			zap.String("stream.state", strState),
		)
	}

	s.state = state
}

func (s *Stream) Start() error {
	if s.state != STREAM_STATE_NONE {
		return errors.New("stream state is already started")
	}
	s.setState(STREAM_STATE_STARTING)
	s.startDeferedSaveTimer()
	s.setState(STREAM_STATE_RUNNING)
	return nil
}

func (s *Stream) Close() error {
	if s.state != STREAM_STATE_RUNNING {
		return errors.New("stream state is not running")
	}
	s.setState(STREAM_STATE_STOPPING)
	// Stop the DeferedCommand.
	// Save & flush messages from ingest buffer.
	// It waits until Run function finished.
	close(s.done)
	s.wg.Wait()
	s.setState(STREAM_STATE_NONE)
	return nil
}

func (s *Stream) AddIterator(it *StreamIterator) error {
	if s.state != STREAM_STATE_RUNNING {
		return errors.New("stream state is not running")
	}

	itUUID := it.GetUUID()
	s.iterators[itUUID] = it
	s.logger.Info(
		"Add stream iterator",
		zap.String("topic", "stream"),
		zap.String("method", "AddIterator"),
		zap.String("stream.uuid", s.info.UUID.String()),
		zap.String("it.uuid", itUUID.String()),
	)
	return nil
}

func (s *Stream) CloseIterators() error {
	s.logger.Debug(
		"Close stream iterators",
		zap.String("topic", "stream"),
		zap.String("method", "CloseIterator"),
		zap.String("stream.uuid", s.info.UUID.String()),
	)

	for _, it := range s.iterators {
		if err := it.Close(); err != nil {
			return err
		}
	}

	// empty all iterators
	s.iterators = make(StreamIteratorMap)
	return nil
}

func (s *Stream) CloseIterator(iterUUID StreamIteratorUUID) error {
	var it *StreamIterator
	var found bool
	if it, found = s.iterators[iterUUID]; !found {
		// maybe the iterator has timed out or has been already be deleted
		return errors.New("iterator not found")
	}

	s.logger.Info(
		"Close stream iterator",
		zap.String("topic", "stream"),
		zap.String("method", "CloseIterator"),
		zap.String("stream.uuid", s.info.UUID.String()),
		zap.String("it.uuid", iterUUID.String()),
	)

	// close the iterator
	if err := it.Close(); err != nil {
		return err
	}

	// clean
	delete(s.iterators, iterUUID)
	it = nil
	return nil
}

func (s *Stream) GetIterator(iterUUID StreamIteratorUUID) (*StreamIterator, error) {
	if it, found := s.iterators[iterUUID]; !found {
		return nil, fmt.Errorf("iterator not found: %s", iterUUID.String())
	} else {
		return it, nil
	}
}

func (s *Stream) GetRecords(c *fasthttp.RequestCtx, iterUUID StreamIteratorUUID, maxRecords uint) (*GetStreamRecordsResponse, error) {
	if s.state != STREAM_STATE_RUNNING {
		return nil, errors.New("stream state is not running")
	}

	if it, found := s.iterators[iterUUID]; !found {
		// maybe the iterator has timed out and be deleted
		return nil, errors.New("iterator not found")
	} else {
		return it.GetRecords(c, maxRecords)
	}
}

func (s *Stream) PutMessage(c *fasthttp.RequestCtx, message map[string]interface{}) (MessageId, error) {
	if s.state != STREAM_STATE_RUNNING {
		return 0, errors.New("stream state is not running")
	}
	s.muIncMsgId.Lock()
	s.info.LastMsgId += 1
	msgId := s.info.LastMsgId
	s.muIncMsgId.Unlock()
	s.ingestBuffer.PutMessage(msgId, time.Now(), message)
	return msgId, nil
}

func (s *Stream) PutMessages(c *fasthttp.RequestCtx, records []interface{}) ([]MessageId, error) {
	if s.state != STREAM_STATE_RUNNING {
		return nil, errors.New("stream state is not running")
	}
	msgIds := make([]MessageId, len(records))
	for i, message := range records {
		s.muIncMsgId.Lock()
		s.info.LastMsgId += 1
		msgId := s.info.LastMsgId
		s.muIncMsgId.Unlock()
		msgIds[i] = msgId
		s.ingestBuffer.PutMessage(msgId, time.Now(), message)
	}
	return msgIds, nil
}

func (s *Stream) startDeferedSaveTimer() {
	s.wg.Add(1)
	go func() {
		s.Run()
		s.wg.Done()
	}()
}

func (s *Stream) Run() {
	s.logger.Debug(
		"Starting stream",
		zap.String("topic", "stream"),
		zap.String("method", "Run"),
		zap.String("stream.uuid", s.info.UUID.String()),
	)
	defer s.logger.Debug(
		"Stream stopped",
		zap.String("topic", "stream"),
		zap.String("method", "Run"),
		zap.String("stream.uuid", s.info.UUID.String()),
	)
	var (
		timer           *time.Timer
		flushC          <-chan time.Time
		immediateExecCh chan DeferedStreamRecord
		deferedExecCh   chan DeferedStreamRecord
		err             error
	)

	bulkFlushFrequency := s.ingestBuffer.GetBulkFlushFrequency()
	if bulkFlushFrequency <= 0 {
		immediateExecCh = s.ingestBuffer.GetChannelMsg()
	} else {
		deferedExecCh = s.ingestBuffer.GetChannelMsg()
	}

	for {
		select {
		case <-s.done:
			if s.logVerbosity > 0 {
				s.logger.Debug(
					"Stopping stream...",
					zap.String("topic", "stream"),
					zap.String("method", "Run"),
					zap.String("stream.uuid", s.info.UUID.String()),
				)
			}
			if timer != nil {
				timer.Stop()
				timer = nil
			}
			if err = s.ingestBuffer.Save(); err != nil {
				s.logger.Error(
					"Can't save stream ingest buffer",
					zap.String("topic", "stream"),
					zap.String("method", "Run"),
					zap.String("stream.uuid", s.info.UUID.String()),
					zap.Error(err),
				)
			}
			if err = s.ingestBuffer.Close(); err != nil {
				s.logger.Error(
					"Can't close stream ingest buffer",
					zap.String("topic", "stream"),
					zap.String("method", "Run"),
					zap.String("stream.uuid", s.info.UUID.String()),
					zap.Error(err),
				)
			}
			if err = s.CloseIterators(); err != nil {
				s.logger.Error(
					"Can't close stream iterators",
					zap.String("topic", "stream"),
					zap.String("method", "Run"),
					zap.String("stream.uuid", s.info.UUID.String()),
					zap.Error(err),
				)
			}
			return

		case command := <-immediateExecCh:
			// no flush timeout configured. Immediately execute command
			s.bufferizeMessage(command, true)

		case command := <-deferedExecCh:
			// flush timeout configured. Only update internal state and track pending
			// updates to be written to registry.
			s.bufferizeMessage(command, false)
			if flushC == nil {
				timer = time.NewTimer(bulkFlushFrequency)
				flushC = timer.C
			}

		case <-flushC:
			timer.Stop()
			if err = s.ingestBuffer.Save(); err != nil {
				s.logger.Error(
					"Can't save stream ingest buffer",
					zap.String("topic", "stream"),
					zap.String("method", "Run"),
					zap.String("stream.uuid", s.info.UUID.String()),
					zap.Error(err),
				)
			}
			flushC = nil
			timer = nil
		}
	}
}

func (s *Stream) bufferizeMessage(msg DeferedStreamRecord, immediateSave bool) {
	if s.logVerbosity > 1 {
		s.logger.Debug(
			"bufferizeMessage",
			zap.String("topic", "stream"),
			zap.String("method", "bufferizeMessage"),
			zap.String("stream.uuid", s.info.UUID.String()),
		)
	}
	s.ingestBuffer.AppendMesssage(msg)
	if immediateSave || s.ingestBuffer.IsFull() {
		if err := s.ingestBuffer.Save(); err != nil {
			s.logger.Error(
				"Can't save stream ingest buffer",
				zap.String("topic", "stream"),
				zap.String("method", "bufferizeMessage"),
				zap.String("stream.uuid", s.info.UUID.String()),
				zap.Error(err),
			)
		}
	}
}

func (s *Stream) Log() {
	s.logger.Info("Stream",
		zap.String("topic", "stream"),
		zap.String("method", "Log"),
		zap.String("stream.uuid", s.info.UUID.String()),
		zap.Time("stream.creationDate", s.info.CreationDate),
		zap.Time("stream.lastUpdate", s.info.LastUpdate),
		zap.Uint64("stream.cptMessages", uint64(s.info.CptMessages)),
		zap.String("stream.cptMessagesHumanized", humanize.Comma(int64(s.info.CptMessages))),
		zap.Uint64("stream.sizeInBytes", uint64(s.info.SizeInBytes)),
		zap.String("stream.sizeHumanized", humanize.Bytes(uint64(s.info.SizeInBytes))),
		zap.Any("stream.properties", s.info.Properties),
	)
}

func (s *Stream) UpdateProperties(properties *StreamProperties) {
	if s.logVerbosity > 0 {
		s.logger.Debug("UpdateProperties")
	}
	s.info.UpdateProperties(properties)
}

func (s *Stream) SetProperties(properties *StreamProperties) {
	if s.logVerbosity > 0 {
		s.logger.Debug("SetProperties")
	}
	s.info.SetProperties(properties)
}

func (s *Stream) GetProperties() *StreamProperties {
	return &s.info.Properties
}

func (s *Stream) MatchFilterProperties(jqFilter *gojq.Query) (bool, error) {
	result, err := s.info.MatchFilterProperties(jqFilter)
	if err != nil {
		s.logger.Error(
			"jq error",
			zap.String("topic", "stream"),
			zap.String("method", "MatchFilterProperties"),
			zap.String("stream.uuid", s.info.UUID.String()),
			zap.String("jq", jqFilter.String()),
			zap.Error(err),
		)
	}
	return result, err
}

func (s *Stream) GetInfo() *StreamInfo {
	return s.info
}

func (s *Stream) GetUUID() StreamUUID {
	return s.info.UUID
}

func (s *Stream) GetIteratorsCount() int {
	return len(s.iterators)
}

func NewStream(info *StreamInfo, ingestBuffer *buffering.StreamIngestBuffer, logger *zap.Logger, logVerbosity int) *Stream {
	return &Stream{
		info:         info,
		iterators:    make(StreamIteratorMap),
		logger:       logger,
		logVerbosity: logVerbosity,
		ingestBuffer: ingestBuffer,
		done:         make(chan struct{}),
		wg:           sync.WaitGroup{},
		state:        STREAM_STATE_NONE,
	}
}
