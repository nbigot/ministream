package service

import (
	"errors"
	"sync"
	"time"

	"github.com/nbigot/ministream/buffering"
	"github.com/nbigot/ministream/config"
	"github.com/nbigot/ministream/constants"
	"github.com/nbigot/ministream/log"
	"github.com/nbigot/ministream/storageprovider"
	"github.com/nbigot/ministream/storageprovider/registry"
	. "github.com/nbigot/ministream/stream"
	. "github.com/nbigot/ministream/types"
	. "github.com/nbigot/ministream/web/apierror"

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

func (svc *Service) Init() error {
	return svc.sp.Init()
}

func (svc *Service) GetStreamsCount() int {
	return len(svc.Hashmap)
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
		time.Duration(svc.conf.Streams.BulkFlushFrequency)*time.Second,
		svc.conf.Streams.BulkMaxSize,
		svc.conf.Streams.ChannelBufferSize,
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
	if svc.conf.Streams.MaxAllowedStreams > 0 && uint(svc.GetStreamsCount()) >= svc.conf.Streams.MaxAllowedStreams {
		err := errors.New("cannot create stream, limit reached")
		svc.logger.Error(
			"Cannot create stream",
			zap.String("topic", "stream"),
			zap.String("method", "CreateStream"),
			zap.Error(err),
		)
		return nil, err
	}

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
	return svc.sp.SaveStreamCatalog()
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

	if s, found := svc.Hashmap[uuid]; found {
		svc.mapMutex.RUnlock()
		return s
	}

	svc.mapMutex.RUnlock()
	return nil
}

func (svc *Service) GetStreamsUUIDs() StreamUUIDList {
	svc.mapMutex.RLock()

	uuids := make([]StreamUUID, 0, len(svc.Hashmap))
	for k := range svc.Hashmap {
		uuids = append(uuids, k)
	}

	svc.mapMutex.RUnlock()
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

	// check limit the number of iterators for the stream
	if svc.conf.Streams.MaxIteratorsPerStream > 0 && streamPtr.GetIteratorsCount() > svc.conf.Streams.MaxIteratorsPerStream {
		return errorCreateRecordsIterator(streamUUID, constants.ErrorCantCreateRecordsIterator, errors.New("too many iterators opened for this stream"))
	}

	iteratorUUID := uuid.New()
	if handler, err = svc.sp.NewStreamIteratorHandler(streamUUID, iteratorUUID); err != nil {
		return errorCreateRecordsIterator(streamUUID, constants.ErrorCantCreateRecordsIterator, err)
	}

	if iter, err = NewStreamIterator(streamUUID, iteratorUUID, req, handler, svc.GetLogger()); err != nil {
		return errorCreateRecordsIterator(streamUUID, constants.ErrorInvalidCreateRecordsIteratorRequest, err)
	}

	if err = streamPtr.AddIterator(iter); err != nil {
		return errorCreateRecordsIterator(streamUUID, constants.ErrorCantCreateRecordsIterator, err)
	}

	if err = iter.Open(); err != nil {
		return errorCreateRecordsIterator(streamUUID, constants.ErrorCantCreateRecordsIterator, err)
	}

	return iteratorUUID, nil
}

func (svc *Service) GetLogger() *zap.Logger {
	return svc.logger
}

func (svc *Service) BuildIndex(streamUUID StreamUUID) (interface{}, error) {
	return svc.sp.BuildIndex(streamUUID)
}

func (svc *Service) Finalize() {
	svc.Stop()
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
	_ = svc.sp.Stop()
	svc.Hashmap = make(StreamMap)
}

func (svc *Service) setStreamMap(streamUUID StreamUUID, s *Stream) {
	svc.mapMutex.Lock()
	if s == nil {
		delete(svc.Hashmap, streamUUID)
	} else {
		svc.Hashmap[streamUUID] = s
	}
	svc.mapMutex.Unlock()
}

func NewService(conf *config.Config) *Service {
	var err error

	svc, err := NewStreamService(log.Logger, conf)
	if err != nil {
		log.Logger.Fatal("Error while instantiate stream service",
			zap.String("topic", "server"),
			zap.String("method", "NewService"),
			zap.Error(err),
		)
	}

	err = svc.Init()
	if err != nil {
		log.Logger.Fatal("Error while initialize stream service",
			zap.String("topic", "server"),
			zap.String("method", "NewService"),
			zap.Error(err),
		)
	}

	_, err = svc.LoadStreams()
	if err != nil {
		log.Logger.Fatal("Error while loading streams",
			zap.String("topic", "server"),
			zap.String("method", "NewService"),
			zap.Error(err),
		)
	}

	log.Logger.Info(
		"Stream server started",
		zap.String("topic", "server"),
		zap.String("method", "NewService"),
	)

	return svc
}

func NewStreamService(logger *zap.Logger, conf *config.Config) (*Service, error) {
	sp, err := registry.NewStorageProvider(conf)
	if err != nil {
		return nil, err
	}
	return &Service{logger: logger, conf: conf, sp: sp, Hashmap: make(StreamMap)}, nil
}

func errorCreateRecordsIterator(streamUUID uuid.UUID, errorCode int, err error) (StreamIteratorUUID, *APIError) {
	return uuid.Nil, &APIError{
		Message:    "cannot create stream iterator",
		Details:    err.Error(),
		Code:       errorCode,
		HttpCode:   fiber.StatusBadRequest,
		StreamUUID: streamUUID,
		Err:        err,
	}
}
