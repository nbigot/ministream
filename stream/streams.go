package stream

import (
	"errors"
	"io/ioutil"
	"ministream/config"
	"ministream/log"
	"os"
	"sync"

	"github.com/goccy/go-json"
	"github.com/google/uuid"
	"github.com/itchyny/gojq"
	"go.uber.org/zap"
)

type StreamMap = map[StreamUUID]*Stream

type StreamsDB struct {
	Hashmap StreamMap
	//Account      *Account
	StreamsMutex sync.Mutex
	logger       *zap.Logger
}

type streamListSerializeStruct struct {
	StreamsUUID []StreamUUID `json:"streams"`
}

var Streams StreamsDB

func init() {
	Streams.Hashmap = make(StreamMap)
}

func (db *StreamsDB) SetLogger(logger *zap.Logger) {
	db.logger = logger
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

func (db *StreamsDB) LoadStreams(filename string) error {
	db.StreamsMutex.Lock()
	defer db.StreamsMutex.Unlock()

	db.logger.Info(
		"Loading stream catalog",
		zap.String("topic", "stream"),
		zap.String("method", "LoadStreams"),
		zap.String("filename", filename),
	)

	file, err := os.Open(filename)
	if err != nil {
		db.logger.Fatal(
			"Can't open stream catalog",
			zap.String("topic", "stream"),
			zap.String("method", "LoadStreams"),
			zap.String("filename", filename),
			zap.Error(err),
		)
	}
	defer file.Close()

	s := streamListSerializeStruct{}
	jsonDecoder := json.NewDecoder(file)
	err = jsonDecoder.Decode(&s)
	if err != nil {
		db.logger.Fatal(
			"Can't decode json stream list",
			zap.String("topic", "stream"),
			zap.String("method", "LoadStreams"),
			zap.String("filename", filename),
			zap.Error(err),
		)
	}

	if len(s.StreamsUUID) > 0 {
		db.logger.Info(
			"Found streams",
			zap.String("topic", "stream"),
			zap.String("method", "LoadStreams"),
			zap.Int("streams", len(s.StreamsUUID)),
		)
		err = db.loadFromUUIDs(s.StreamsUUID)
		if err != nil {
			return err
		}
	} else {
		db.logger.Info(
			"No stream found",
			zap.String("topic", "stream"),
			zap.String("method", "LoadStreams"),
		)
	}

	return err
}

func (db *StreamsDB) CreateStream(properties *StreamProperties) (*Stream, error) {
	db.StreamsMutex.Lock()
	defer db.StreamsMutex.Unlock()

	uuid := uuid.New()
	stream := createStream(uuid, log.Logger)
	stream.SetProperties(properties)
	db.Hashmap[stream.UUID] = stream
	db.logger.Info(
		"Create stream",
		zap.String("topic", "stream"),
		zap.String("method", "CreateStream"),
		zap.String("stream.uuid", uuid.String()),
	)
	// create directory
	if err := stream.CreateDirectory(); err != nil {
		return nil, err
	}
	// save stream
	if err := stream.save(); err != nil {
		return stream, err
	}
	// save stream list
	//if err := CronJobStreamsSaver.SendRequest(true); err != nil {
	if err := db.saveStreamCatalog(config.Configuration.StreamsFile); err != nil {
		return stream, err
	}
	// save Catalog
	stream.StartDeferedSaveTimer()
	return stream, nil
}

func (db *StreamsDB) saveStreamCatalog(filename string) error {
	db.logger.Info(
		"Save stream catalog",
		zap.String("topic", "stream"),
		zap.String("method", "saveStreamCatalog"),
		zap.String("filename", filename),
	)

	s := streamListSerializeStruct{
		StreamsUUID: *db.GetStreamsUUIDs(),
	}
	if sJson, err := json.Marshal(s); err != nil {
		db.logger.Fatal(
			"Can't serialize json stream list",
			zap.String("topic", "stream"),
			zap.String("method", "saveStreamCatalog"),
			zap.String("filename", filename),
			zap.Error(err),
		)
	} else {
		if err := ioutil.WriteFile(filename, sJson, 0644); err != nil {
			db.logger.Fatal(
				"Can't save stream catalog",
				zap.String("topic", "stream"),
				zap.String("method", "saveStreamCatalog"),
				zap.String("filename", filename),
				zap.Error(err),
			)
		}
	}

	return nil
}

func (db *StreamsDB) DeleteStream(uuid StreamUUID) error {
	db.StreamsMutex.Lock()
	defer db.StreamsMutex.Unlock()

	stream := db.GetStream(uuid)
	if stream == nil {
		err := errors.New("Stream not found")
		db.logger.Error(
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
	delete(db.Hashmap, uuid)
	db.logger.Info(
		"Stream deleted",
		zap.String("topic", "stream"),
		zap.String("method", "DeleteStream"),
		zap.String("StreamUUID", uuid.String()),
	)
	return nil
}

func (db *StreamsDB) GetStream(uuid StreamUUID) *Stream {
	if stream, found := db.Hashmap[uuid]; found {
		return stream
	}

	return nil
}

func (db *StreamsDB) GetStreamsUUIDs() *[]StreamUUID {
	uuids := make([]StreamUUID, 0, len(db.Hashmap))
	for k := range db.Hashmap {
		uuids = append(uuids, k)
	}
	return &uuids
}

func (db *StreamsDB) GetStreamsUUIDsFiltered(jqFilter ...*gojq.Query) *[]StreamUUID {
	uuids := make([]StreamUUID, 0, len(db.Hashmap))
	for uuid := range db.Hashmap {
		s := db.GetStream(uuid)
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
	return &uuids
}

func (db *StreamsDB) GetStreamsFiltered(jqFilter ...*gojq.Query) *[]*Stream {
	rows := make([]*Stream, 0, len(db.Hashmap))
	for _, s := range db.Hashmap {
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

func (db *StreamsDB) loadFromUUIDs(uuids []StreamUUID) error {
	for _, uuid := range uuids {
		stream, err := LoadStreamFromUUID(uuid, db.logger)
		if err != nil {
			return err
		}
		db.Hashmap[uuid] = stream
		stream.Log()
		stream.StartDeferedSaveTimer()
	}

	return nil
}
