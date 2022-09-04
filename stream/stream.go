package stream

import (
	"errors"
	"ministream/buffering"
	"ministream/config"
	. "ministream/types"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/itchyny/gojq"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

type Stream struct {
	info         *StreamInfo
	logger       *zap.Logger
	iterators    StreamIteratorMap
	ingestBuffer *buffering.StreamIngestBuffer
	muIncMsgId   sync.Mutex
	done         chan struct{}
	wg           sync.WaitGroup
}

func (s *Stream) AddIterator(it *StreamIterator) error {
	// check limit the number of iterators for the stream
	if config.Configuration.Streams.MaxIteratorsPerStream > 0 && len(s.iterators) > config.Configuration.Streams.MaxIteratorsPerStream {
		return errors.New("too many iterators opened for this stream")
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
	it.Close()

	// clean
	delete(s.iterators, iterUUID)
	it = nil
	return nil
}

func (s *Stream) GetIterator(iterUUID StreamIteratorUUID) (*StreamIterator, error) {
	if it, found := s.iterators[iterUUID]; !found {
		return nil, errors.New("iterator not found")
	} else {
		return it, nil
	}
}

func (s *Stream) GetRecords(c *fasthttp.RequestCtx, iterUUID StreamIteratorUUID, maxRecords int) (*GetStreamRecordsResponse, error) {
	if it, found := s.iterators[iterUUID]; !found {
		// maybe the iterator has timed out and be deleted
		return nil, errors.New("iterator not found")
	} else {
		return it.GetRecords(c, maxRecords)
	}

	// response := GetStreamRecordsResponse{
	// 	Status:             "",
	// 	Duration:           0,
	// 	Count:              0,
	// 	CountErrors:        0,
	// 	CountSkipped:       0,
	// 	Remain:             false,
	// 	StreamUUID:         s.info.UUID,
	// 	StreamIteratorUUID: iterUUID,
	// 	Records:            make([]interface{}, 0),
	// }

	// defer func() {
	// 	response.Duration = time.Since(startTime).Milliseconds()
	// }()

	// if err = it.Seek(s.GetIndex()); err != nil { // ??? arg en trop?
	// 	response.Status = "error"
	// 	return &response, err
	// }

	// it.Stats.LastTimeRead = time.Now()
	// reader := bufio.NewReaderSize(it.file, 1024*1024)
	// reader.Reset(it.file)
	// for {
	// 	line, err := reader.ReadString('\n')
	// 	if err != nil {
	// 		// err is ofter io.EOF (end of file reached)
	// 		// err may also raise when EOL char was not found
	// 		if err == io.EOF {
	// 			it.SaveSeek()
	// 			response.Status = "success"
	// 			return &response, nil
	// 		} else {
	// 			it.SaveSeek()
	// 			response.CountErrors += 1
	// 			response.Status = "error"
	// 			return nil, err
	// 		}
	// 	}

	// 	it.Stats.RecordsRead++
	// 	it.Stats.BytesRead += int64(len(line))

	// 	var message interface{}
	// 	err2 := json.Unmarshal([]byte(line), &message)
	// 	if err2 != nil {
	// 		s.logger.Error(
	// 			"json format error",
	// 			zap.String("topic", "stream"),
	// 			zap.String("method", "GetRecords"),
	// 			zap.String("stream.uuid", s.info.UUID.String()),
	// 			zap.String("line", line),
	// 			zap.Error(err),
	// 		)
	// 		response.CountErrors += 1
	// 		continue
	// 	}

	// 	if it.jqFilter == nil {
	// 		response.Count += 1
	// 		response.Records = append(response.Records, message)
	// 	} else {
	// 		// apply filter on message

	// 		// bash example: echo {"foo": 0} | jq .foo
	// 		// bash example: echo [{"foo": 0}] | jq .[0]
	// 		// bash example: echo [{"foo": 0}] | jq .[0].foo
	// 		// ".foo | .."
	// 		// TODO: iterator checkpoint?
	// 		// TODO: save iterator last seek file?

	// 		jqIter := it.jqFilter.RunWithContext(c, message)
	// 		v, ok := jqIter.Next()
	// 		if ok {
	// 			// the message is matching the jq filter
	// 			response.Count += 1
	// 			response.Records = append(response.Records, v)
	// 		} else {
	// 			if err, isAnError := v.(error); isAnError {
	// 				// invlid (TODO: decide to keep or to skip the message)
	// 				s.logger.Error(
	// 					"jq error",
	// 					zap.String("topic", "stream"),
	// 					zap.String("method", "GetRecords"),
	// 					zap.String("stream.uuid", s.info.UUID.String()),
	// 					zap.String("jq", it.jqFilter.String()),
	// 					zap.Error(err),
	// 				)
	// 				response.CountErrors += 1
	// 			} else {
	// 				// does not match the jq filter therefore skip the message
	// 				response.CountSkipped += 1
	// 			}
	// 		}
	// 	}

	// 	if len(response.Records) >= maxRecords {
	// 		response.Remain = true
	// 		break
	// 	}
	// }

	// it.Stats.RecordsErrors += response.CountErrors
	// it.Stats.RecordsSkipped += response.CountSkipped
	// it.Stats.RecordsSent += response.Count
	// it.SaveSeek()
	// response.Status = "success"
	// return &response, nil
}

func (s *Stream) PutMessage(c *fasthttp.RequestCtx, message map[string]interface{}) (MessageId, error) {
	s.muIncMsgId.Lock()
	s.info.LastMsgId += 1
	msgId := s.info.LastMsgId
	s.muIncMsgId.Unlock()
	s.ingestBuffer.PutMessage(msgId, time.Now(), message)
	return msgId, nil
}

func (s *Stream) PutMessages(c *fasthttp.RequestCtx, records []interface{}) ([]MessageId, error) {
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

func (s *Stream) StartDeferedSaveTimer() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.Run()
	}()
}

// Stop stops the DeferedCommand. It waits until Run function finished.
func (s *Stream) StopDeferedSaveTimer() {
	s.logger.Info(
		"Stopping DeferedCommand",
		zap.String("topic", "stream"),
		zap.String("method", "StopDeferedSaveTimer"),
		zap.String("stream.uuid", s.info.UUID.String()),
	)
	defer s.logger.Info(
		"DeferedCommand stopped",
		zap.String("topic", "stream"),
		zap.String("method", "StopDeferedSaveTimer"),
		zap.String("stream.uuid", s.info.UUID.String()),
	)

	close(s.done)
	s.wg.Wait()
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
			s.logger.Info(
				"Stopping stream...",
				zap.String("topic", "stream"),
				zap.String("method", "Run"),
				zap.String("stream.uuid", s.info.UUID.String()),
			)
			return

		case command := <-immediateExecCh:
			// no flush timeout configured. Immediatly execute command
			s.bufferizeMesssage(command, true)

		case command := <-deferedExecCh:
			// flush timeout configured. Only update internal state and track pending
			// updates to be written to registry.
			s.bufferizeMesssage(command, false)
			if flushC == nil {
				timer = time.NewTimer(bulkFlushFrequency)
				flushC = timer.C
			}

		case <-flushC:
			timer.Stop()
			s.ingestBuffer.Save()
			flushC = nil
			timer = nil
		}
	}
}

func (s *Stream) bufferizeMesssage(msg DeferedStreamRecord, immediateSave bool) {
	s.logger.Debug(
		"bufferizeMesssage",
		zap.String("topic", "stream"),
		zap.String("method", "bufferizeMesssage"),
		zap.String("stream.uuid", s.info.UUID.String()),
	)
	s.ingestBuffer.AppendMesssage(msg)
	if immediateSave || s.ingestBuffer.IsFull() {
		s.ingestBuffer.Save()
	}
}

// func (s *Stream) SaveStream() error {
// 	s.logger.Debug(
// 		"save stream",
// 		zap.String("topic", "stream"),
// 		zap.String("method", "save"),
// 		zap.String("stream.uuid", s.info.UUID.String()),
// 	)
// 	s.ingestBuffer.Lock()
// 	defer s.ingestBuffer.Unlock()

// 	// Open stream data file
// 	filename := s.GetDataFilePath()
// 	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
// 	if err != nil {
// 		s.logger.Error(
// 			"can't open data file",
// 			zap.String("topic", "stream"),
// 			zap.String("method", "save"),
// 			zap.String("stream.uuid", s.info.UUID.String()),
// 			zap.Any("filename", filename),
// 			zap.Error(err),
// 		)
// 		return err
// 	}
// 	defer file.Close()

// 	// Open stream index file
// 	filenameIndex := s.GetIndexFilePath()
// 	fileIndex, err := os.OpenFile(filenameIndex, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
// 	if err != nil {
// 		s.logger.Error(
// 			"can't open index file",
// 			zap.String("topic", "stream"),
// 			zap.String("method", "save"),
// 			zap.String("stream.uuid", s.info.UUID.String()),
// 			zap.Any("filename", filenameIndex),
// 			zap.Error(err),
// 		)
// 		return err
// 	}
// 	defer fileIndex.Close()

// 	// Append records to file
// 	for _, msg := range s.ingestBuffer.GetBuffer() {
// 		s.logger.Debug(
// 			"save msg",
// 			zap.String("topic", "stream"),
// 			zap.String("method", "save"),
// 			zap.String("stream.uuid", s.info.UUID.String()),
// 			zap.Any("msg", msg),
// 		)

// 		// serialize message into string
// 		bytes, err := json.Marshal(msg)
// 		if err != nil {
// 			s.logger.Error(
// 				"json",
// 				zap.String("topic", "stream"),
// 				zap.String("method", "save"),
// 				zap.String("stream.uuid", s.info.UUID.String()),
// 				zap.Any("msg", msg),
// 				zap.Error(err),
// 			)
// 			// drop message (should never happen)
// 			return err
// 		}

// 		// append message to file
// 		strjson := string(bytes)
// 		var countBytesWritten int
// 		if countBytesWritten, err = file.WriteString(strjson + "\n"); err != nil {
// 			return err
// 		}
// 		s.info.CptMessages += 1
// 		s.info.LastUpdate = msg.CreationDate

// 		// update index file
// 		// row is (<msg id>, <msg length in bytes>, <date>)
// 		//var data [8]byte
// 		//data[0] = countBytesWritten
// 		//binary.LittleEndian.PutUint32(data, countBytesWritten)
// 		//binary.LittleEndian.PutUint64(data, uint64(i))
// 		// data []byte
// 		// var b []byte
// 		// hdr := (*unsafeheader.Slice)(unsafe.Pointer(&b))
// 		// hdr.Data = (*unsafeheader.String)(unsafe.Pointer(&s)).Data
// 		// hdr.Cap = len(s)
// 		// hdr.Len = len(s)

// 		// pour convertir dans l'autre sens: t := time.Unix(1334289777, 0)

// 		var data = streamRowIndex{msg.Id, int32(countBytesWritten), msg.CreationDate.Unix()}
// 		if err := binary.Write(fileIndex, binary.LittleEndian, data); err != nil {
// 			return err
// 		}

// 		return nil
// 	}

// 	s.ingestBuffer.Clear()

// 	// update fileinfo
// 	fileInfo, err := os.Stat(filename)
// 	if err == nil {
// 		s.FileInfoData = fileInfo
// 		s.info.SizeInBytes = uint64(fileInfo.Size())
// 	}

// 	s.saveFileMetaInfo()

// 	return nil
// }

// func (s *Stream) saveFileMetaInfo() error {
// 	s.logger.Debug(
// 		"saveFileMetaInfo",
// 		zap.String("topic", "stream"),
// 		zap.String("method", "saveFileMetaInfo"),
// 		zap.String("stream.uuid", s.info.UUID.String()),
// 	)
// 	filename := s.GetMetaDataFilePath()
// 	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0644)
// 	if err != nil {
// 		s.logger.Error(
// 			"can't open meta info file",
// 			zap.String("topic", "stream"),
// 			zap.String("method", "saveFileMetaInfo"),
// 			zap.String("stream.uuid", s.info.UUID.String()),
// 			zap.Any("filename", filename),
// 			zap.Error(err),
// 		)
// 		return err
// 	}
// 	defer file.Close()
// 	// serialize message into string
// 	bytes, err := json.Marshal(s)
// 	if err != nil {
// 		s.logger.Error(
// 			"json marshal",
// 			zap.String("topic", "stream"),
// 			zap.String("method", "saveFileMetaInfo"),
// 			zap.String("stream.uuid", s.info.UUID.String()),
// 			zap.Any("stream", s),
// 			zap.Error(err),
// 		)
// 		// skip saving (should never happen)
// 		return err
// 	}
// 	strjson := string(bytes)
// 	if _, err := file.WriteString(strjson); err != nil {
// 		return err
// 	}
// 	return nil
// }

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
	s.logger.Debug("UpdateProperties")
	s.info.UpdateProperties(properties)
}

func (s *Stream) SetProperties(properties *StreamProperties) {
	s.logger.Debug("SetProperties")
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

func NewStream(info *StreamInfo, ingestBuffer *buffering.StreamIngestBuffer, logger *zap.Logger) *Stream {
	return &Stream{
		info:         info,
		iterators:    make(StreamIteratorMap),
		logger:       logger,
		ingestBuffer: ingestBuffer,
		done:         make(chan struct{}),
		wg:           sync.WaitGroup{},
	}
}
