package types

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/itchyny/gojq"
)

func TestNewRecordValueCanBeFilteredWithJq(t *testing.T) {
	// gojq panics on the values it does not support (ex: "invalid type: uint64" with the
	// function type), the record must be made of generic JSON values only
	query, err := gojq.Parse(`[(.i | type), (.i >= 2), (.d | type), (.m.a + 1)]`)
	if err != nil {
		t.Fatal(err)
	}
	date := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

	for _, id := range []MessageId{1, 2, math.MaxInt64, math.MaxUint64} {
		value := NewRecordValue(id, date, map[string]interface{}{"a": 1.0})
		result, ok := query.Run(value).Next()
		if !ok {
			t.Fatalf("id %d: jq returned no result", id)
		}
		if err, isErr := result.(error); isErr {
			t.Fatalf("id %d: jq error: %v", id, err)
		}
		expected := []interface{}{"number", id >= 2, "string", 2.0}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("id %d: expected %v, got %v", id, expected, result)
		}
		if value["d"] != "2026-10-15T12:00:00Z" {
			t.Errorf("id %d: unexpected date %v", id, value["d"])
		}
	}
}
