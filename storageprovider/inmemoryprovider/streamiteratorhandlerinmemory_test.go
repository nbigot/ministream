package inmemoryprovider

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/itchyny/gojq"
	"github.com/nbigot/ministream/types"
	"go.uber.org/zap"
)

func newTestStream(t *testing.T, count int) *InMemoryStream {
	s, err := NewInMemoryStream(types.NewStreamInfo(uuid.New()), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	for i := 1; i <= count; i++ {
		record := types.DeferedStreamRecord{
			Id:           types.MessageId(i),
			CreationDate: start.Add(time.Duration(i) * time.Second),
			Msg:          map[string]interface{}{"n": float64(i)},
		}
		if err := s.AddRecord(&record); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func readIds(t *testing.T, s *InMemoryStream, request *types.StreamIteratorRequest) []types.MessageId {
	h := NewStreamIteratorHandlerInMemory(uuid.New(), uuid.New(), s, zap.NewNop())
	if err := h.Open(); err != nil {
		t.Fatal(err)
	}
	if err := h.Seek(request); err != nil {
		t.Fatal(err)
	}
	ids := []types.MessageId{}
	for {
		id, record, found, _, err := h.GetNextRecord()
		if err != nil {
			t.Fatal(err)
		}
		if !found {
			return ids
		}
		if record.(map[string]interface{})["i"] != id {
			t.Fatalf("record %v has not the id %d", record, id)
		}
		ids = append(ids, id)
	}
}

func TestInMemoryIteratorPositions(t *testing.T) {
	s := newTestStream(t, 3)
	for _, c := range []struct {
		request  types.StreamIteratorRequest
		expected []types.MessageId
	}{
		{types.StreamIteratorRequest{IteratorType: "FIRST_MESSAGE"}, []types.MessageId{1, 2, 3}},
		{types.StreamIteratorRequest{IteratorType: "LAST_MESSAGE"}, []types.MessageId{3}},
		{types.StreamIteratorRequest{IteratorType: "AFTER_LAST_MESSAGE"}, []types.MessageId{}},
		{types.StreamIteratorRequest{IteratorType: "AT_MESSAGE_ID", MessageId: 2}, []types.MessageId{2, 3}},
		{types.StreamIteratorRequest{IteratorType: "AFTER_MESSAGE_ID", MessageId: 2}, []types.MessageId{3}},
	} {
		ids := readIds(t, s, &c.request)
		if len(ids) != len(c.expected) {
			t.Fatalf("%s: expected %v, got %v", c.request.IteratorType, c.expected, ids)
		}
		for i := range ids {
			if ids[i] != c.expected[i] {
				t.Fatalf("%s: expected %v, got %v", c.request.IteratorType, c.expected, ids)
			}
		}
	}
}

func TestInMemoryIteratorAfterLastMessageReadsTheNextRecords(t *testing.T) {
	s := newTestStream(t, 2)
	h := NewStreamIteratorHandlerInMemory(uuid.New(), uuid.New(), s, zap.NewNop())
	if err := h.Seek(&types.StreamIteratorRequest{IteratorType: "AFTER_LAST_MESSAGE"}); err != nil {
		t.Fatal(err)
	}
	record := types.DeferedStreamRecord{Id: 3, CreationDate: time.Now(), Msg: "new"}
	if err := s.AddRecord(&record); err != nil {
		t.Fatal(err)
	}
	id, _, found, _, _ := h.GetNextRecord()
	if !found || id != 3 {
		t.Fatalf("expected the record 3, got found=%v id=%d", found, id)
	}
}

func TestInMemoryRecordsCanBeFilteredWithJq(t *testing.T) {
	s := newTestStream(t, 1)
	h := NewStreamIteratorHandlerInMemory(uuid.New(), uuid.New(), s, zap.NewNop())
	if err := h.Seek(&types.StreamIteratorRequest{IteratorType: "FIRST_MESSAGE"}); err != nil {
		t.Fatal(err)
	}
	_, record, _, _, _ := h.GetNextRecord()

	query, err := gojq.Parse(`[.i, .m.n, (.d | type)]`)
	if err != nil {
		t.Fatal(err)
	}
	v, ok := query.Run(record).Next()
	if !ok {
		t.Fatalf("no output")
	}
	if err, isErr := v.(error); isErr {
		t.Fatalf("jq error: %v", err)
	}
	result := v.([]interface{})
	if result[0] != 1 || result[1] != 1.0 || result[2] != "string" {
		t.Fatalf("unexpected result: %v", result)
	}
}
