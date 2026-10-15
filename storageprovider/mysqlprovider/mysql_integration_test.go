package mysqlprovider

// These tests need a MySQL server, they run only when the environment variable
// MINISTREAM_TEST_MYSQL_DSN is set, for example:
//
//	docker run --name mysql-ministream -e MYSQL_ROOT_PASSWORD=my-secret-pw -e MYSQL_DATABASE=ministream -p 3306:3306 -d mysql:8
//	MINISTREAM_TEST_MYSQL_DSN="root:my-secret-pw@tcp(127.0.0.1:3306)/ministream" go test ./storageprovider/mysqlprovider/
//
// The schema ministream_gotest is dropped and created again by the tests.

import (
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nbigot/ministream/config"
	"github.com/nbigot/ministream/types"
	"go.uber.org/zap"
)

const testSchemaName = "ministream_gotest"

func getTestDSN(t *testing.T) string {
	dsn := os.Getenv("MINISTREAM_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("MINISTREAM_TEST_MYSQL_DSN is not set: test skipped")
	}
	return dsn
}

func dropTestSchema(t *testing.T, dsn string) {
	pool, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = pool.Close()
	}()
	if _, err = pool.Exec("DROP SCHEMA IF EXISTS " + testSchemaName); err != nil {
		t.Fatal(err)
	}
}

// newTestStorage starts a storage provider (a new instance acts like a server restart)
func newTestStorage(t *testing.T, dsn string) *MySQLStorage {
	conf := &config.Config{}
	conf.Storage.MySQL.DataSourceName = dsn
	conf.Storage.MySQL.MaxIdleConns = 2
	conf.Storage.MySQL.MaxOpenConns = 2
	conf.Storage.MySQL.SchemaName = testSchemaName
	sp, err := NewStorageProvider(zap.NewNop(), conf)
	if err != nil {
		t.Fatal(err)
	}
	s := sp.(*MySQLStorage)
	if err = s.Init(); err != nil {
		t.Fatal(err)
	}
	return s
}

func writeTestRecords(t *testing.T, s *MySQLStorage, info *types.StreamInfo, ids ...types.MessageId) {
	w, err := s.NewStreamWriter(info)
	if err != nil {
		t.Fatal(err)
	}
	records := make([]types.DeferedStreamRecord, 0, len(ids))
	for _, id := range ids {
		// dates in UTC like the dates of a server running in a docker container
		records = append(records, types.DeferedStreamRecord{Id: id, CreationDate: time.Now().UTC(), Msg: map[string]interface{}{"a": id}})
		info.IngestedMessages.CptMessages++
		info.IngestedMessages.LastMsgId = id
	}
	if err = w.Write(&records); err != nil {
		t.Fatalf("write records %v: %v", ids, err)
	}
}

func readTestRecords(t *testing.T, s *MySQLStorage, streamUUID types.StreamUUID, request *types.StreamIteratorRequest) []types.MessageId {
	h, err := s.NewStreamIteratorHandler(streamUUID, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if err = h.Seek(request); err != nil {
		t.Fatal(err)
	}
	ids := make([]types.MessageId, 0)
	for {
		id, value, found, _, err := h.GetNextRecord()
		if err != nil {
			t.Fatal(err)
		}
		if !found {
			return ids
		}
		record, ok := value.(map[string]interface{})
		if !ok || record["i"] != id || record["m"] == nil {
			t.Fatalf("unexpected record %d: %#v", id, value)
		}
		ids = append(ids, id)
	}
}

func TestMySQLStreamLifecycle(t *testing.T) {
	dsn := getTestDSN(t)
	dropTestSchema(t, dsn)
	t.Cleanup(func() { dropTestSchema(t, dsn) })

	s := newTestStorage(t, dsn)

	// create a stream with dates in UTC (MySQL rejected their RFC3339 format "...Z")
	info := types.NewStreamInfo(s.GenerateNewStreamUuid())
	info.CreationDate = info.CreationDate.UTC()
	info.LastUpdate = info.LastUpdate.UTC()
	info.Properties["name"] = "test"
	info.IngestedMessages.FirstMsgId = 1
	if err := s.OnCreateStream(info); err != nil {
		t.Fatalf("create stream: %v", err)
	}

	// a failed write must not leave its transaction (and its connection) open
	w, _ := s.NewStreamWriter(info)
	invalid := []types.DeferedStreamRecord{{Id: 1, CreationDate: time.Now(), Msg: make(chan int)}}
	if err := w.Write(&invalid); err == nil {
		t.Fatal("expected an error for a record that cannot be serialized")
	}
	if inUse := s.pool.Stats().InUse; inUse != 0 {
		t.Fatalf("expected no connection in use after a failed write, got %d", inUse)
	}

	writeTestRecords(t, s, info, 1, 2)
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}

	// restart: the stream is loaded from the catalog, its dates are in UTC
	s = newTestStorage(t, dsn)
	defer func() {
		_ = s.Stop()
	}()
	infos, err := s.LoadStreams()
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 1 || infos[0].UUID != info.UUID {
		t.Fatalf("expected the stream %s, got %v", info.UUID, infos)
	}
	info = infos[0]
	if info.ReadableMessages.CptMessages != 2 || info.ReadableMessages.LastMsgId != 2 || info.Properties["name"] != "test" {
		t.Fatalf("unexpected stream info after restart: %+v", info)
	}

	// write after the restart
	writeTestRecords(t, s, info, 3)

	if ids := readTestRecords(t, s, info.UUID, &types.StreamIteratorRequest{IteratorType: "FIRST_MESSAGE"}); len(ids) != 3 || ids[0] != 1 || ids[2] != 3 {
		t.Errorf("FIRST_MESSAGE: expected records [1 2 3], got %v", ids)
	}
	if ids := readTestRecords(t, s, info.UUID, &types.StreamIteratorRequest{IteratorType: "AFTER_MESSAGE_ID", MessageId: 1}); len(ids) != 2 || ids[0] != 2 {
		t.Errorf("AFTER_MESSAGE_ID 1: expected records [2 3], got %v", ids)
	}

	stats, err := s.BuildIndex(info.UUID)
	if err != nil {
		t.Fatalf("build index: %v", err)
	}
	if stats.(*StreamIndexStats).CptMessages != 3 {
		t.Errorf("build index: expected 3 messages, got %+v", stats)
	}

	if err = s.DeleteStream(info.UUID); err != nil {
		t.Fatalf("delete stream: %v", err)
	}
	var tables int
	if err = s.pool.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = ? AND table_name LIKE 'stream\\_%'", testSchemaName).Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables != 0 {
		t.Errorf("expected the stream table to be dropped, %d stream table(s) left", tables)
	}
}
