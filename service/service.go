package service

import (
	"errors"
	"ministream/buffering"
	"ministream/config"
	"ministream/constants"
	"ministream/log"
	"ministream/storageprovider"
	. "ministream/stream"
	. "ministream/types"
	. "ministream/web/apierror"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/itchyny/gojq"
	"go.uber.org/zap"
)

type StreamMap = map[StreamUUID]*Stream

type Service struct {
	Hashmap  StreamMap
	mapMutex sync.RWMutex
	logger   *zap.Logger
	sp       storageprovider.IStorageProvider
	conf     *config.Config
}

func (svc *Service) Init() {
	svc.sp.Init()
}

func (svc *Service) startStream(info *StreamInfo) (*Stream, error) {
	var err error
	var writer buffering.IStreamWriter
	if writer, err = svc.sp.NewStreamWriter(info); err != nil {
		return nil, err
	}
	if err = writer.Init(); err != nil {
		return nil, err
	}
	if err = writer.Open(); err != nil {
		return nil, err
	}

	ingestBuffer := buffering.NewStreamIngestBuffer(
		time.Duration(config.Configuration.Streams.BulkFlushFrequency)*time.Second,
		config.Configuration.Streams.BulkMaxSize,
		config.Configuration.Streams.ChannelBufferSize,
		writer,
	)
	s := NewStream(info, ingestBuffer, log.Logger, svc.conf.Streams.LogVerbosity)

	svc.setStreamMap(s.GetUUID(), s)
	svc.logger.Info(
		"Start stream",
		zap.String("topic", "stream"),
		zap.String("method", "startStream"),
		zap.String("stream.uuid", info.UUID.String()),
	)

	return s, s.Start()
}

func (svc *Service) LoadStreams() (StreamInfoList, error) {
	streamInfoList, err := svc.sp.LoadStreams()
	if err != nil {
		return streamInfoList, err
	}

	var errStartStream error = nil
	wg := sync.WaitGroup{}
	for _, streamInfo := range streamInfoList {
		wg.Add(1)
		go func(info *StreamInfo) {
			if _, err := svc.startStream(info); err != nil {
				errStartStream = err
			}
			wg.Done()
		}(streamInfo)
	}
	wg.Wait()

	return streamInfoList, errStartStream
}

func (svc *Service) CreateStream(properties *StreamProperties) (*Stream, error) {
	uuid := svc.sp.GenerateNewStreamUuid()

	svc.logger.Info(
		"Create stream",
		zap.String("topic", "stream"),
		zap.String("method", "CreateStream"),
		zap.String("stream.uuid", uuid.String()),
	)

	var err error
	info := NewStreamInfo(uuid)
	info.Properties = *properties

	if err = svc.sp.OnCreateStream(info); err != nil {
		return nil, err
	}

	var s *Stream
	if s, err = svc.startStream(info); err != nil {
		return s, err
	}

	if err = svc.saveStreamCatalog(); err != nil {
		return s, err
	}

	return s, nil
}

func (svc *Service) saveStreamCatalog() error {
	// save stream list
	return svc.sp.SaveStreamCatalog(svc.GetStreamsUUIDs())
}

func (svc *Service) DeleteStream(streamUUID StreamUUID) error {
	var err error

	s := svc.GetStream(streamUUID)
	if s == nil {
		err = errors.New("stream not found")
		svc.logger.Error(
			"Cannot delete stream",
			zap.String("topic", "stream"),
			zap.String("method", "DeleteStream"),
			zap.String("StreamUUID", streamUUID.String()),
			zap.Error(err),
		)
		return err
	}

	if err = s.Close(); err != nil {
		return err
	}
	if err = svc.sp.DeleteStream(streamUUID); err != nil {
		return err
	}

	// delete uuid from hashmap
	svc.setStreamMap(streamUUID, nil)

	if err = svc.saveStreamCatalog(); err != nil {
		return err
	}

	svc.logger.Info(
		"Stream deleted",
		zap.String("topic", "stream"),
		zap.String("method", "DeleteStream"),
		zap.String("StreamUUID", streamUUID.String()),
	)
	return nil
}

func (svc *Service) GetStream(uuid StreamUUID) *Stream {
	svc.mapMutex.RLock()
	defer svc.mapMutex.RUnlock()

	if s, found := svc.Hashmap[uuid]; found {
		return s
	}

	return nil
}

func (svc *Service) GetStreamsUUIDs() StreamUUIDList {
	svc.mapMutex.RLock()
	defer svc.mapMutex.RUnlock()

	uuids := make([]StreamUUID, 0, len(svc.Hashmap))
	for k := range svc.Hashmap {
		uuids = append(uuids, k)
	}
	return uuids
}

func (svc *Service) GetStreamsUUIDsFiltered(jqFilter ...*gojq.Query) StreamUUIDList {
	svc.mapMutex.RLock()
	defer svc.mapMutex.RUnlock()

	uuids := make([]StreamUUID, 0, len(svc.Hashmap))
	for uuid := range svc.Hashmap {
		s := svc.GetStream(uuid)
		if s != nil {
			match_filters := true
			for _, jq := range jqFilter {
				if jq != nil {
					match, err := s.MatchFilterProperties(jq)
					if err != nil || !match {
						match_filters = false
						break
					}
				}
			}
			if match_filters {
				uuids = append(uuids, uuid)
			}
		}
	}
	return uuids
}

func (svc *Service) GetStreamsFiltered(jqFilter ...*gojq.Query) *[]*Stream {
	svc.mapMutex.RLock()
	defer svc.mapMutex.RUnlock()
	// TODO: should be able to filter on meta data also
	// [.result.rows[] | select((.creationDate >= "2022-10-09T00:00:00.0000000+02:00") and (.cptMessages > 10000))] | sort_by(.sizeInBytes)
	// TODO: rename cptMessages into cptRecords
	rows := make([]*Stream, 0, len(svc.Hashmap))
	for _, s := range svc.Hashmap {
		if s != nil {
			match_filters := true
			for _, jq := range jqFilter {
				if jq != nil {
					match, err := s.MatchFilterProperties(jq)
					if err != nil || !match {
						match_filters = false
						break
					}
				}
			}
			if match_filters {
				rows = append(rows, s)
			}
		}
	}
	return &rows
}

func (svc *Service) CreateRecordsIterator(streamPtr *Stream, req *StreamIteratorRequest) (StreamIteratorUUID, *APIError) {
	var err error
	var iter *StreamIterator
	var handler IStreamIteratorHandler
	streamUUID := streamPtr.GetUUID()
	iteratorUUID := uuid.New()
	if handler, err = svc.sp.NewStreamIteratorHandler(streamUUID, iteratorUUID); err != nil {
		return uuid.Nil, &APIError{
			Message:    "cannot create stream iterator",
			Details:    err.Error(),
			Code:       constants.ErrorCantCreateRecordsIterator,
			HttpCode:   fiber.StatusBadRequest,
			StreamUUID: streamUUID,
			Err:        err,
		}
	}

	if iter, err = NewStreamIterator(streamUUID, iteratorUUID, req, handler, svc.GetLogger()); err != nil {
		return uuid.Nil, &APIError{
			Message:    "cannot create stream iterator",
			Details:    err.Error(),
			Code:       constants.ErrorInvalidCreateRecordsIteratorRequest,
			HttpCode:   fiber.StatusBadRequest,
			StreamUUID: streamUUID,
			Err:        err,
		}
	}

	if err = streamPtr.AddIterator(iter); err != nil {
		return uuid.Nil, &APIError{
			Message:    "cannot create stream iterator",
			Details:    err.Error(),
			Code:       constants.ErrorCantCreateRecordsIterator,
			HttpCode:   fiber.StatusBadRequest,
			StreamUUID: streamUUID,
			Err:        err,
		}
	}

	if err = iter.Open(); err != nil {
		return uuid.Nil, &APIError{
			Message:    "cannot open stream iterator",
			Details:    err.Error(),
			Code:       constants.ErrorCantCreateRecordsIterator,
			HttpCode:   fiber.StatusBadRequest,
			StreamUUID: streamUUID,
			Err:        err,
		}
	}

	return iteratorUUID, nil
}

func (svc *Service) GetLogger() *zap.Logger {
	return svc.logger
}

func (svc *Service) BuildIndex(streamUUID StreamUUID) (interface{}, error) {
	return svc.sp.BuildIndex(streamUUID)
}

func (svc *Service) Stop() {
	svc.mapMutex.RLock()
	defer svc.mapMutex.RUnlock()

	wg := sync.WaitGroup{}
	for _, streamPtr := range svc.Hashmap {
		wg.Add(1)
		go func(s *Stream) {
			s.Close()
			wg.Done()
		}(streamPtr)
	}
	wg.Wait()
	svc.sp.Stop()
	svc.Hashmap = make(StreamMap)
}

func (svc *Service) setStreamMap(streamUUID StreamUUID, s *Stream) {
	svc.mapMutex.Lock()
	defer svc.mapMutex.Unlock()

	if s == nil {
		delete(svc.Hashmap, streamUUID)
	} else {
		svc.Hashmap[streamUUID] = s
	}
}

func NewService() *Service {
	var err error
	svc, err := NewStreamService(log.Logger, &config.Configuration)
	if err != nil {
		log.Logger.Fatal("Error while instantiate stream service",
			zap.String("topic", "server"),
			zap.String("method", "GoServer"),
			zap.Error(err),
		)
	}
	svc.Init()
	_, err = svc.LoadStreams()
	if err != nil {
		log.Logger.Fatal("Error while loading streams",
			zap.String("topic", "server"),
			zap.String("method", "GoServer"),
			zap.Error(err),
		)
	}

	log.Logger.Info(
		"Stream server started",
		zap.String("topic", "server"),
		zap.String("method", "GoServer"),
	)
	return svc
}

func NewStreamService(logger *zap.Logger, conf *config.Config) (*Service, error) {
	sp, err := storageprovider.NewStorageProvider(logger, conf)
	if err != nil {
		return nil, err
	}
	return &Service{logger: logger, conf: conf, sp: sp, Hashmap: make(StreamMap)}, nil
}
