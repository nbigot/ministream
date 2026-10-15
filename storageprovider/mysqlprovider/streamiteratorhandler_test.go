package mysqlprovider

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/itchyny/gojq"
	"github.com/nbigot/ministream/types"
	"go.uber.org/zap"
)

func TestGetNextRecordCanBeFilteredWithJq(t *testing.T) {
	// the record is read from the buffer of the iterator (no SQL query)
	date := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	h := NewStreamIteratorHandlerMySQL(uuid.New(), uuid.New(), nil, "ministream", "stream_test", nil, 100, zap.NewNop())
	h.buffer = append(h.buffer, rowMySQL{Id: 4, Msg: types.DeferedStreamRecord{Id: 4, CreationDate: date, Msg: map[string]interface{}{"a": 4.0}}})

	id, value, found, _, err := h.GetNextRecord()
	if err != nil || !found || id != 4 {
		t.Fatalf("expected record 4, got id=%d found=%v err=%v", id, found, err)
	}

	query, err := gojq.Parse("[.i, .d, .m.a]")
	if err != nil {
		t.Fatal(err)
	}
	result, ok := query.Run(value).Next()
	if !ok {
		t.Fatal("jq returned no result")
	}
	if err, isErr := result.(error); isErr {
		t.Fatalf("jq error: %v", err)
	}
	expected := []interface{}{4, "2026-10-15T12:00:00Z", 4.0}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}
