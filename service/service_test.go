package service

import (
	"ministream/config"
	"ministream/startup"
	"ministream/stream"
	"ministream/types"
	"testing"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

func BenchmarkSetStreamMap(b *testing.B) {
	startup.SetupStorageProviders()
	s := stream.NewStream(nil, nil, nil, 0)

	streamUUIDs := make([]types.StreamUUID, b.N)
	for n := 0; n < b.N; n++ {
		streamUUIDs[n] = uuid.New()
	}

	bench_config := []byte(`
storage:
    type: "InMemory"
    inmemory:
        maxRecordsByStream: 0
        maxSize: "1gb"
    logger:
        level: "info"
        encoding: "json"
`)

	var conf config.Config
	var err error
	err = yaml.Unmarshal(bench_config, &conf)
	if err != nil {
		panic("error while parsing configuration:" + err.Error())
	}

	svc, err := NewStreamService(nil, &conf)
	if err != nil {
		panic("error while creating service:" + err.Error())
	}

	for n := 0; n < b.N; n++ {
		svc.setStreamMap(streamUUIDs[n], s)
	}

	for n := 0; n < b.N; n++ {
		svc.setStreamMap(streamUUIDs[n], nil)
	}
}
