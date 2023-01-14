package service

import (
	"testing"

	"github.com/nbigot/ministream/config"
	"github.com/nbigot/ministream/storageprovider/registry"
	"github.com/nbigot/ministream/stream"
	"github.com/nbigot/ministream/types"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

func initConfig() *config.Config {

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

	if err := yaml.Unmarshal(bench_config, &conf); err != nil {
		panic("error while parsing configuration:" + err.Error())
	}
	return &conf
}

func BenchmarkSetStreamMap(b *testing.B) {
	if err := registry.SetupStorageProviders(); err != nil {
		panic("error while setup storage providers:" + err.Error())
	}

	svc, err := NewStreamService(nil, initConfig())
	if err != nil {
		panic("error while creating service:" + err.Error())
	}
	s := stream.NewStream(nil, nil, nil, 0)

	streamUUIDs := make([]types.StreamUUID, b.N)
	for n := 0; n < b.N; n++ {
		streamUUIDs[n] = uuid.New()
	}

	for n := 0; n < b.N; n++ {
		svc.setStreamMap(streamUUIDs[n], s)
	}

	for n := 0; n < b.N; n++ {
		svc.setStreamMap(streamUUIDs[n], nil)
	}
}

func TestGetStream(t *testing.T) {
	if err := registry.SetupStorageProviders(); err != nil {
		t.Fatalf("error while setup storage providers:" + err.Error())
	}

	svc, err := NewStreamService(nil, initConfig())
	if err != nil {
		t.Fatalf("error while creating service:" + err.Error())
	}

	streamId := uuid.New()
	if s := svc.GetStream(streamId); s != nil {
		t.Fatalf("expected nil value")
	}

	s := stream.NewStream(nil, nil, nil, 0)
	svc.setStreamMap(streamId, s)
	if sResult := svc.GetStream(streamId); sResult != s {
		t.Fatalf("wrong value")
	}
}
