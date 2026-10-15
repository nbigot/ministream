package service

import (
	"os"
	"testing"
	"time"

	"github.com/itchyny/gojq"
	"github.com/nbigot/ministream/config"
	"github.com/nbigot/ministream/storageprovider/registry"
	"github.com/nbigot/ministream/stream"
	"github.com/nbigot/ministream/types"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

func TestMain(m *testing.M) {
	if err := registry.SetupStorageProviders(); err != nil {
		panic("error while setup storage providers:" + err.Error())
	}
	os.Exit(m.Run())
}

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
	svc, err := NewStreamService(nil, initConfig())
	if err != nil {
		t.Fatalf("error while creating service: %v", err)
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

func TestGetStreamsUUIDsFiltered(t *testing.T) {
	svc, err := NewStreamService(nil, initConfig())
	if err != nil {
		t.Fatalf("error while creating service: %v", err)
	}

	newStream := func(name string) types.StreamUUID {
		info := types.NewStreamInfo(uuid.New())
		info.Properties["name"] = name
		svc.setStreamMap(info.UUID, stream.NewStream(info, nil, nil, 0))
		return info.UUID
	}
	matching := newStream("test")
	newStream("other")

	jq, err := gojq.Parse(`.name == "test"`)
	if err != nil {
		t.Fatalf("invalid jq filter: %v", err)
	}

	result := make(chan types.StreamUUIDList, 1)
	go func() { result <- svc.GetStreamsUUIDsFiltered(jq) }()
	select {
	case uuids := <-result:
		if len(uuids) != 1 || uuids[0] != matching {
			t.Fatalf("unexpected result: %v", uuids)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("GetStreamsUUIDsFiltered is blocked (deadlock)")
	}
}
