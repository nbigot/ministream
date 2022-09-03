package stream

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"ministream/config"
	"os"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/goccy/go-json"
	"github.com/google/uuid"
	"github.com/itchyny/gojq"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

type MessageId = uint64
type Size64 = uint64
type StreamUUID = uuid.UUID
type StreamProperties = map[string]interface{}

// Defered stream message to be saved
type DeferedStreamMessage struct {
	Id           MessageId   `json:"i"`
	CreationDate time.Time   `json:"d"`
	Msg          interface{} `json:"m"`
}

type Stream struct {
	FilePath     string           `json:"filepath"`
	UUID         StreamUUID       `json:"uuid" example:"4ce589e2-b483-467b-8b59-758b339801db"`
	CptMessages  Size64           `json:"cptMessages" example:"12345"`
	SizeInBytes  Size64           `json:"sizeInBytes" example:"4567890"`
	CreationDate time.Time        `json:"creationDate"`
	LastUpdate   time.Time        `json:"lastUpdate"`
	Properties   StreamProperties `json:"properties"`
	LastMsgId    MessageId        `json:"lastMsgId"`
	FileInfoData os.FileInfo      `json:"-"`
	muIncMsgId   sync.Mutex       `json:"-"`
	logger       *zap.Logger      `json:"-"`
	iterators    StreamIteratorMap
	index        *StreamIndex
	// variables used for defered save
	bulkFlushFrequency   time.Duration // RecordMaxBufferedTime
	bulkMaxSize          int
	channelMsg           chan DeferedStreamMessage
	msgBuffer            []DeferedStreamMessage
	bufferedStateUpdates Size64
	mu                   sync.Mutex
	// shutdown handling
	done chan struct{}
	wg   sync.WaitGroup
}

type streamRowIndex struct {
	msgId      MessageId
	bytesCount int32
	dateTime   int64
}

func LoadStreamFromUUID(uuid StreamUUID, logger *zap.Logger) (*Stream, error) {
	logger.Info("Loading stream", zap.String("topic", "stream"),
		zap.String("method", "LoadStreamFromUUID"), zap.String("stream.uuid", uuid.String()))
	stream := createStream(uuid, logger)

	var filename = ConvertUUIDToMetaFilePath(uuid)
	file, err := os.Open(filename)
	if err != nil {
		logger.Error("Can't open stream",
			zap.String("topic", "stream"),
			zap.String("method", "LoadStreamFromUUID"),
			zap.String("filename", filename), zap.Error(err),
		)
		return nil, err
	}
	defer file.Close()

	jsonDecoder := json.NewDecoder(file)
	err = jsonDecoder.Decode(&stream)
	if err != nil {
		logger.Error("Can't decode json stream",
			zap.String("topic", "stream"),
			zap.String("method", "LoadStreamFromUUID"),
			zap.String("filename", filename), zap.Error(err),
		)
		return nil, err
	}

	return stream, nil
}

func createStream(uuid uuid.UUID, logger *zap.Logger) *Stream {
	return &Stream{
		FilePath:             ConvertUUIDToMetaFilePath(uuid),
		UUID:                 StreamUUID(uuid),
		CreationDate:         time.Now(),
		LastUpdate:           time.Now(),
		LastMsgId:            0,
		CptMessages:          0,
		SizeInBytes:          0,
		Properties:           StreamProperties{},
		iterators:            make(StreamIteratorMap),
		index:                nil,
		logger:               logger,
		bulkFlushFrequency:   time.Duration(config.Configuration.Streams.BulkFlushFrequency) * time.Second,
		bulkMaxSize:          config.Configuration.Streams.BulkMaxSize,
		msgBuffer:            make([]DeferedStreamMessage, 0, config.Configuration.Streams.BulkMaxSize),
		bufferedStateUpdates: 0,
		channelMsg:           make(chan DeferedStreamMessage, config.Configuration.Streams.ChannelBufferSize),
		done:                 make(chan struct{}),
		wg:                   sync.WaitGroup{},
	}
}

func (s *Stream) GetDirectoryPath() string {
	return fmt.Sprintf("%sstreams/%s", config.Configuration.DataDirectory, s.UUID.String())
}

func ConvertUUIDToMetaFilePath(uuid uuid.UUID) string {
	return fmt.Sprintf("%sstreams/%s/stream.json", config.Configuration.DataDirectory, uuid.String())
}

func (s *Stream) GetMetaDataFilePath() string {
	return fmt.Sprintf("%sstreams/%s/stream.json", config.Configuration.DataDirectory, s.UUID.String())
}

func (s *Stream) GetDataFilePath() string {
	return fmt.Sprintf("%sstreams/%s/data.jsonl", config.Configuration.DataDirectory, s.UUID.String())
}

func (s *Stream) GetIndexFilePath() string {
	return fmt.Sprintf("%sstreams/%s/index.bin", config.Configuration.DataDirectory, s.UUID.String())
}

func (s *Stream) CreateDirectory() error {
	return os.MkdirAll(s.GetDirectoryPath(), os.ModePerm)
}

func (s *Stream) AddIterator(it *StreamIterator) error {
	// check limit the number of iterators for the stream
	if config.Configuration.Streams.MaxIteratorsPerStream > 0 && len(s.iterators) > config.Configuration.Streams.MaxIteratorsPerStream {
		return errors.New("too many iterators opened for this stream")
	}

	it.filename = s.GetDataFilePath()
	err := it.Open()
	if err != nil {
		s.logger.Error(
			"can't open data file",
			zap.String("topic", "stream"),
			zap.String("method", "AddIterator"),
			zap.Any("filename", it.filename),
			zap.Error(err),
		)
		return err
	}

	s.iterators[it.UUID] = it
	s.logger.Info(
		"Add stream iterator",
		zap.String("topic", "stream"),
		zap.String("method", "AddIterator"),
		zap.String("stream.uuid", s.UUID.String()),
		zap.String("it.uuid", it.UUID.String()),
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
		zap.String("stream.uuid", s.UUID.String()),
		zap.String("it.uuid", it.UUID.String()),
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

func (s *Stream) GetMessages(c *fasthttp.RequestCtx, iterUUID StreamIteratorUUID, maxRecords int) (*GetStreamRecordsResponse, error) {
	var err error
	var it *StreamIterator
	var found bool
	startTime := time.Now()

	if it, found = s.iterators[iterUUID]; !found {
		// maybe the iterator has timed out and be deleted
		return nil, errors.New("iterator not found")
	}

	response := GetStreamRecordsResponse{
		Status:             "",
		Duration:           0,
		Count:              0,
		CountErrors:        0,
		CountSkipped:       0,
		Remain:             false,
		StreamUUID:         s.UUID,
		StreamIteratorUUID: iterUUID,
		Records:            make([]interface{}, 0),
	}

	defer func() {
		response.Duration = time.Since(startTime).Milliseconds()
	}()

	if err = it.Seek(s.GetIndex()); err != nil {
		response.Status = "error"
		return &response, err
	}

	it.Stats.LastTimeRead = time.Now()
	reader := bufio.NewReaderSize(it.file, 1024*1024)
	reader.Reset(it.file)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			// err is ofter io.EOF (end of file reached)
			// err may also raise when EOL char was not found
			if err == io.EOF {
				it.SaveSeek()
				response.Status = "success"
				return &response, nil
			} else {
				it.SaveSeek()
				response.CountErrors += 1
				response.Status = "error"
				return nil, err
			}
		}

		it.Stats.RecordsRead++
		it.Stats.BytesRead += int64(len(line))

		var message interface{}
		err2 := json.Unmarshal([]byte(line), &message)
		if err2 != nil {
			s.logger.Error(
				"json format error",
				zap.String("topic", "stream"),
				zap.String("method", "GetMessages"),
				zap.String("stream.uuid", s.UUID.String()),
				zap.String("line", line),
				zap.Error(err),
			)
			response.CountErrors += 1
			continue
		}

		if it.jqFilter == nil {
			response.Count += 1
			response.Records = append(response.Records, message)
		} else {
			// apply filter on message

			// bash example: echo {"foo": 0} | jq .foo
			// bash example: echo [{"foo": 0}] | jq .[0]
			// bash example: echo [{"foo": 0}] | jq .[0].foo
			// ".foo | .."
			// TODO: iterator checkpoint?
			// TODO: save iterator last seek file?

			jqIter := it.jqFilter.RunWithContext(c, message)
			v, ok := jqIter.Next()
			if ok {
				// the message is matching the jq filter
				response.Count += 1
				response.Records = append(response.Records, v)
			} else {
				if err, isAnError := v.(error); isAnError {
					// invlid (TODO: decide to keep or to skip the message)
					s.logger.Error(
						"jq error",
						zap.String("topic", "stream"),
						zap.String("method", "GetMessages"),
						zap.String("stream.uuid", s.UUID.String()),
						zap.String("jq", it.jqFilter.String()),
						zap.Error(err),
					)
					response.CountErrors += 1
				} else {
					// does not match the jq filter therefore skip the message
					response.CountSkipped += 1
				}
			}
		}

		if len(response.Records) >= maxRecords {
			response.Remain = true
			break
		}
	}

	it.Stats.RecordsErrors += response.CountErrors
	it.Stats.RecordsSkipped += response.CountSkipped
	it.Stats.RecordsSent += response.Count
	it.SaveSeek()
	response.Status = "success"
	return &response, nil
}

func (s *Stream) PutMessage(c *fasthttp.RequestCtx, message map[string]interface{}) (MessageId, error) {
	s.muIncMsgId.Lock()
	s.LastMsgId += 1
	msgId := s.LastMsgId
	s.muIncMsgId.Unlock()
	s.channelMsg <- DeferedStreamMessage{Id: msgId, CreationDate: time.Now(), Msg: message}
	return msgId, nil
}

func (s *Stream) PutMessages(c *fasthttp.RequestCtx, records []interface{}) ([]MessageId, error) {
	msgIds := make([]MessageId, len(records))
	for i, msg := range records {
		s.muIncMsgId.Lock()
		s.LastMsgId += 1
		msgId := s.LastMsgId
		s.muIncMsgId.Unlock()
		msgIds[i] = msgId
		s.channelMsg <- DeferedStreamMessage{Id: msgId, CreationDate: time.Now(), Msg: msg}
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
		zap.String("stream.uuid", s.UUID.String()),
	)
	defer s.logger.Info(
		"DeferedCommand stopped",
		zap.String("topic", "stream"),
		zap.String("method", "StopDeferedSaveTimer"),
		zap.String("stream.uuid", s.UUID.String()),
	)

	close(s.done)
	s.wg.Wait()
}

func (s *Stream) Run() {
	s.logger.Debug(
		"Starting stream",
		zap.String("topic", "stream"),
		zap.String("method", "Run"),
		zap.String("stream.uuid", s.UUID.String()),
	)
	defer s.logger.Debug(
		"Stream stopped",
		zap.String("topic", "stream"),
		zap.String("method", "Run"),
		zap.String("stream.uuid", s.UUID.String()),
	)
	var (
		timer           *time.Timer
		flushC          <-chan time.Time
		immediateExecCh chan DeferedStreamMessage
		deferedExecCh   chan DeferedStreamMessage
	)

	if s.bulkFlushFrequency <= 0 {
		immediateExecCh = s.channelMsg
	} else {
		deferedExecCh = s.channelMsg
	}

	for {
		select {
		case <-s.done:
			s.logger.Info(
				"Stopping stream...",
				zap.String("topic", "stream"),
				zap.String("method", "Run"),
				zap.String("stream.uuid", s.UUID.String()),
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
				timer = time.NewTimer(s.bulkFlushFrequency)
				flushC = timer.C
			}

		case <-flushC:
			timer.Stop()
			s.save()
			flushC = nil
			timer = nil
		}
	}
}

func (s *Stream) bufferizeMesssage(msg DeferedStreamMessage, immediateSave bool) {
	s.logger.Debug(
		"bufferizeMesssage",
		zap.String("topic", "stream"),
		zap.String("method", "bufferizeMesssage"),
		zap.String("stream.uuid", s.UUID.String()),
	)
	s.mu.Lock()
	s.msgBuffer = append(s.msgBuffer, msg)
	s.mu.Unlock()
	if immediateSave || (len(s.msgBuffer) >= s.bulkMaxSize) {
		s.save()
	}
}

func (s *Stream) save() error {
	s.logger.Debug(
		"save stream",
		zap.String("topic", "stream"),
		zap.String("method", "save"),
		zap.String("stream.uuid", s.UUID.String()),
	)
	s.mu.Lock()
	defer s.mu.Unlock()

	// Open stream data file
	filename := s.GetDataFilePath()
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		s.logger.Error(
			"can't open data file",
			zap.String("topic", "stream"),
			zap.String("method", "save"),
			zap.String("stream.uuid", s.UUID.String()),
			zap.Any("filename", filename),
			zap.Error(err),
		)
		return err
	}
	defer file.Close()

	// Open stream index file
	filenameIndex := s.GetIndexFilePath()
	fileIndex, err := os.OpenFile(filenameIndex, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		s.logger.Error(
			"can't open index file",
			zap.String("topic", "stream"),
			zap.String("method", "save"),
			zap.String("stream.uuid", s.UUID.String()),
			zap.Any("filename", filenameIndex),
			zap.Error(err),
		)
		return err
	}
	defer fileIndex.Close()

	// Append records to file
	for _, msg := range s.msgBuffer {
		s.logger.Debug(
			"save msg",
			zap.String("topic", "stream"),
			zap.String("method", "save"),
			zap.String("stream.uuid", s.UUID.String()),
			zap.Any("msg", msg),
		)

		// serialize message into string
		bytes, err := json.Marshal(msg)
		if err != nil {
			s.logger.Error(
				"json",
				zap.String("topic", "stream"),
				zap.String("method", "save"),
				zap.String("stream.uuid", s.UUID.String()),
				zap.Any("msg", msg),
				zap.Error(err),
			)
			// drop message (should never happen)
			return err
		}

		// append message to file
		strjson := string(bytes)
		var countBytesWritten int
		if countBytesWritten, err = file.WriteString(strjson + "\n"); err != nil {
			return err
		}
		s.CptMessages += 1
		s.LastUpdate = msg.CreationDate

		// update index file
		// row is (<msg id>, <msg length in bytes>, <date>)
		//var data [8]byte
		//data[0] = countBytesWritten
		//binary.LittleEndian.PutUint32(data, countBytesWritten)
		//binary.LittleEndian.PutUint64(data, uint64(i))
		// data []byte
		// var b []byte
		// hdr := (*unsafeheader.Slice)(unsafe.Pointer(&b))
		// hdr.Data = (*unsafeheader.String)(unsafe.Pointer(&s)).Data
		// hdr.Cap = len(s)
		// hdr.Len = len(s)

		// pour convertir dans l'autre sens: t := time.Unix(1334289777, 0)

		var data = streamRowIndex{msg.Id, int32(countBytesWritten), msg.CreationDate.Unix()}
		if err := binary.Write(fileIndex, binary.LittleEndian, data); err != nil {
			return err
		}
	}

	// update fileinfo
	fileInfo, err := os.Stat(filename)
	if err == nil {
		s.FileInfoData = fileInfo
		s.SizeInBytes = uint64(fileInfo.Size())
	}

	// clear buffer
	s.msgBuffer = nil

	s.saveFileMetaInfo()

	return nil
}

func (s *Stream) saveFileMetaInfo() error {
	s.logger.Debug(
		"saveFileMetaInfo",
		zap.String("topic", "stream"),
		zap.String("method", "saveFileMetaInfo"),
		zap.String("stream.uuid", s.UUID.String()),
	)
	filename := s.GetMetaDataFilePath()
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		s.logger.Error(
			"can't open meta info file",
			zap.String("topic", "stream"),
			zap.String("method", "saveFileMetaInfo"),
			zap.String("stream.uuid", s.UUID.String()),
			zap.Any("filename", filename),
			zap.Error(err),
		)
		return err
	}
	defer file.Close()
	// serialize message into string
	bytes, err := json.Marshal(s)
	if err != nil {
		s.logger.Error(
			"json marshal",
			zap.String("topic", "stream"),
			zap.String("method", "saveFileMetaInfo"),
			zap.String("stream.uuid", s.UUID.String()),
			zap.Any("stream", s),
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

func (s *Stream) Log() {
	s.logger.Info("Stream",
		zap.String("topic", "stream"),
		zap.String("method", "Log"),
		zap.String("stream.uuid", s.UUID.String()),
		zap.String("stream.filepath", s.FilePath),
		zap.Time("stream.creationDate", s.CreationDate),
		zap.Time("stream.lastUpdate", s.LastUpdate),
		zap.Uint64("stream.cptMessages", uint64(s.CptMessages)),
		zap.String("stream.cptMessagesHumanized", humanize.Comma(int64(s.CptMessages))),
		zap.Uint64("stream.sizeInBytes", uint64(s.SizeInBytes)),
		zap.String("stream.sizeHumanized", humanize.Bytes(uint64(s.SizeInBytes))),
		zap.Any("stream.properties", s.Properties),
	)
}

func (s *Stream) UpdateProperties(properties *StreamProperties) {
	s.logger.Debug("UpdateProperties")

	// add or update properties
	if properties != nil {
		for key, value := range *properties {
			s.Properties[key] = value
		}
	}
}

func (s *Stream) SetProperties(properties *StreamProperties) {
	s.logger.Debug("SetProperties")

	// delete all existing properties
	for k := range s.Properties {
		delete(s.Properties, k)
	}

	// add new properties
	if properties != nil {
		for key, value := range *properties {
			s.Properties[key] = value
		}
	}
}

func (s *Stream) MatchFilterProperties(jqFilter *gojq.Query) (bool, error) {
	jqIter := jqFilter.Run(s.Properties)
	for {
		v, ok := jqIter.Next()
		if !ok {
			return false, nil
		}

		if err, ok := v.(error); ok {
			s.logger.Error(
				"jq error",
				zap.String("topic", "stream"),
				zap.String("method", "MatchFilterProperties"),
				zap.String("stream.uuid", s.UUID.String()),
				zap.String("jq", jqFilter.String()),
				zap.Error(err),
			)
			return false, err
		}

		// if v is true then stream is matching the filter
		if v == true {
			// stream properties are matching the jq filter
			return true, nil
		} else {
			// stream properties does not match the jq filter
			return false, nil
		}
	}
}

func (s *Stream) GetIndex() *StreamIndex {
	if s.index == nil {
		s.index = &StreamIndex{s: s, logger: s.logger, filename: s.GetIndexFilePath(), file: nil}
	}
	return s.index
}

func (s *Stream) RebuildIndex() (*StreamIndexStats, error) {
	idx := s.GetIndex()
	return idx.BuildIndex()
}
