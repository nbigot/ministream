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
	Hashmap StreamMap
	//Account      *Account
	StreamsMutex sync.Mutex
	logger       *zap.Logger
	sp           storageprovider.IStorageProvider
	conf         *config.Config
}

// type streamListSerializeStruct struct {
// 	StreamsUUID []StreamUUID `json:"streams"`
// }

func NewStreamService(logger *zap.Logger, conf *config.Config) (*Service, error) {
	sp, err := storageprovider.NewStorageProvider(logger, conf)
	if err != nil {
		return nil, err
	}
	return &Service{logger: logger, conf: conf, sp: sp, Hashmap: make(StreamMap)}, nil
}

func (svc *Service) Init() {
	svc.sp.Init()
}

// func (db *StreamsDB) LoadAccount(filename string) error {
// 	db.StreamsMutex.Lock()
// 	defer db.StreamsMutex.Unlock()

// 	account, err := LoadAccount(filename)
// 	if err != nil {
// 		return err
// 	}

// 	db.Account = account
// 	return nil
// }

func (svc *Service) LoadStreams() (StreamInfoList, error) {
	svc.StreamsMutex.Lock()
	defer svc.StreamsMutex.Unlock()

	return svc.sp.LoadStreams()
}

func (svc *Service) CreateStream(properties *StreamProperties) (*Stream, error) {
	svc.StreamsMutex.Lock()
	defer svc.StreamsMutex.Unlock()

	var err error
	var writer buffering.IStreamWriter
	uuid := svc.sp.GenerateNewStreamUuid()
	info := NewStreamInfo(uuid)
	if writer, err = svc.sp.NewStreamWriter(info); err != nil {
		return nil, err
	}
	ingestBuffer := buffering.NewStreamIngestBuffer(
		time.Duration(config.Configuration.Streams.BulkFlushFrequency)*time.Second,
		config.Configuration.Streams.BulkMaxSize,
		config.Configuration.Streams.ChannelBufferSize,
		writer,
	)
	s := NewStream(info, ingestBuffer, log.Logger)
	s.SetProperties(properties)

	if err = svc.sp.OnCreateStream(s.GetInfo()); err != nil {
		return nil, err
	}

	svc.Hashmap[s.GetUUID()] = s
	svc.logger.Info(
		"Create stream",
		zap.String("topic", "stream"),
		zap.String("method", "CreateStream"),
		zap.String("stream.uuid", uuid.String()),
	)
	// save stream list
	//if err := CronJobStreamsSaver.SendRequest(true); err != nil {
	if err = svc.saveStreamCatalog(); err != nil {
		return s, err
	}
	// save Catalog
	s.StartDeferedSaveTimer()
	return s, nil
}

func (svc *Service) saveStreamCatalog() error {
	return svc.sp.SaveStreamCatalog(svc.GetStreamsUUIDs())
}

func (svc *Service) DeleteStream(uuid StreamUUID) error {
	svc.StreamsMutex.Lock()
	defer svc.StreamsMutex.Unlock()

	s := svc.GetStream(uuid)
	if s == nil {
		err := errors.New("Stream not found")
		svc.logger.Error(
			"Cannot delete stream",
			zap.String("topic", "stream"),
			zap.String("method", "DeleteStream"),
			zap.String("StreamUUID", uuid.String()),
			zap.Error(err),
		)
		return err
	}

	// TODO:
	// save new stream (create dirs)
	// save Streams
	CronJobStreamsSaver.SendRequest(true)
	// save Catalog
	delete(svc.Hashmap, uuid)
	svc.logger.Info(
		"Stream deleted",
		zap.String("topic", "stream"),
		zap.String("method", "DeleteStream"),
		zap.String("StreamUUID", uuid.String()),
	)
	return nil
}

func (svc *Service) GetStream(uuid StreamUUID) *Stream {
	if s, found := svc.Hashmap[uuid]; found {
		return s
	}

	return nil
}

func (svc *Service) GetStreamsUUIDs() StreamUUIDList {
	uuids := make([]StreamUUID, 0, len(svc.Hashmap))
	for k := range svc.Hashmap {
		uuids = append(uuids, k)
	}
	return uuids
}

func (svc *Service) GetStreamsUUIDsFiltered(jqFilter ...*gojq.Query) StreamUUIDList {
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

func (svc *Service) loadStreamsFromUUIDs(streamUUIDs StreamUUIDList) error {
	var err error
	var info *StreamInfo
	var writer buffering.IStreamWriter

	for _, streamUUID := range streamUUIDs {
		if info, err = svc.sp.LoadStreamFromUUID(streamUUID); err != nil {
			return err
		}
		if writer, err = svc.sp.NewStreamWriter(info); err != nil {
			return err
		}
		ingestBuffer := buffering.NewStreamIngestBuffer(
			time.Duration(config.Configuration.Streams.BulkFlushFrequency)*time.Second,
			config.Configuration.Streams.BulkMaxSize,
			config.Configuration.Streams.ChannelBufferSize,
			writer,
		)
		s := NewStream(info, ingestBuffer, svc.logger)
		svc.Hashmap[streamUUID] = s
		s.Log()
		s.StartDeferedSaveTimer()
	}

	return nil
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
			Code:       constants.ErrorInvalidCreateRecordsIteratorRequest,
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

	//context.Background()
	//context.TODO()
	//ctx, cancel := context.WithCancel(context.Background())

	log.Logger.Info(
		"Stream server started",
		zap.String("topic", "server"),
		zap.String("method", "GoServer"),
	)
	return svc
}
