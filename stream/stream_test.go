package stream

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nbigot/ministream/buffering"
	"github.com/nbigot/ministream/types"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// fakeHandler returns the given records, then no record.
type fakeHandler struct {
	records []interface{}
	next    int
	closed  atomic.Int32
}

func (h *fakeHandler) Open() error                                     { return nil }
func (h *fakeHandler) Close() error                                    { h.closed.Add(1); return nil }
func (h *fakeHandler) Seek(request *types.StreamIteratorRequest) error { return nil }
func (h *fakeHandler) SaveSeek() error                                 { return nil }
func (h *fakeHandler) GetNextRecord() (types.MessageId, interface{}, bool, bool, error) {
	if h.next >= len(h.records) {
		return 0, nil, false, false, nil
	}
	h.next++
	return types.MessageId(h.next), h.records[h.next-1], true, true, nil
}

// fakeWriter stores the written records.
type fakeWriter struct {
	mu      sync.Mutex
	records []types.DeferedStreamRecord
}

func (w *fakeWriter) Init() error  { return nil }
func (w *fakeWriter) Open() error  { return nil }
func (w *fakeWriter) Close() error { return nil }
func (w *fakeWriter) Write(records *[]types.DeferedStreamRecord) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.records = append(w.records, *records...)
	return nil
}

func newIterator(t *testing.T, request *types.StreamIteratorRequest, handler *fakeHandler) *StreamIterator {
	it, err := NewStreamIterator(uuid.New(), uuid.New(), request, handler, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	return it
}

func TestCloseIteratorDuringLongPolling(t *testing.T) {
	handler := &fakeHandler{}
	it := newIterator(t, &types.StreamIteratorRequest{IteratorType: "FIRST_MESSAGE", MaxWaitTimeSeconds: 20}, handler)

	done := make(chan error, 1)
	go func() {
		_, err := it.GetRecords(newRequestCtx(), 10)
		done <- err
	}()
	time.Sleep(200 * time.Millisecond)
	if err := it.Close(); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("the long polling request was not interrupted")
	}
	if handler.closed.Load() != 1 {
		t.Fatalf("the handler must be closed once, closed %d times", handler.closed.Load())
	}
	// a closed iterator cannot be used anymore
	if _, err := it.GetRecords(newRequestCtx(), 10); err == nil {
		t.Fatalf("expected an error")
	}
}

func TestGetRecordsWithJqErrors(t *testing.T) {
	records := []interface{}{
		map[string]interface{}{"a": "text"},
		map[string]interface{}{"a": 1.0},
	}
	it := newIterator(t, &types.StreamIteratorRequest{IteratorType: "FIRST_MESSAGE", JqFilter: ".a + 1"}, &fakeHandler{records: records})
	response, err := it.GetRecords(newRequestCtx(), 10)
	if err != nil {
		t.Fatal(err)
	}
	// the jq runtime error ("text" + 1) is an error, not a record
	if response.Count != 1 || response.CountErrors != 1 || len(response.Records) != 1 {
		t.Fatalf("unexpected response: %+v", response)
	}
	if response.LastRecordIdRead != 2 {
		t.Fatalf("expected last record id 2, got %d", response.LastRecordIdRead)
	}

	// no more record: the last record id read is kept
	response, err = it.GetRecords(newRequestCtx(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if response.Count != 0 || response.LastRecordIdRead != 2 || it.LastRecordIdRead != 2 {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func newRunningStream(t *testing.T, writer *fakeWriter) *Stream {
	ingestBuffer := buffering.NewStreamIngestBuffer(time.Hour, 1000, 1000, writer)
	s := NewStream(types.NewStreamInfo(uuid.New()), ingestBuffer, zap.NewNop(), 0)
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestConcurrentIterators(t *testing.T) {
	s := newRunningStream(t, &fakeWriter{})
	defer func() { _ = s.Close() }()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			it := newIterator(t, &types.StreamIteratorRequest{IteratorType: "FIRST_MESSAGE"}, &fakeHandler{})
			if err := s.AddIterator(it); err != nil {
				t.Error(err)
				return
			}
			_, _ = s.GetRecords(newRequestCtx(), it.GetUUID(), 10)
			_ = s.GetIteratorsCount()
			if err := s.CloseIterator(it.GetUUID()); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if s.GetIteratorsCount() != 0 {
		t.Fatalf("expected no iterator, got %d", s.GetIteratorsCount())
	}
}

func TestPendingMessagesAreSavedWhenTheStreamStops(t *testing.T) {
	writer := &fakeWriter{}
	ingestBuffer := buffering.NewStreamIngestBuffer(time.Hour, 1000, 1000, writer)
	s := NewStream(types.NewStreamInfo(uuid.New()), ingestBuffer, zap.NewNop(), 0)
	// messages waiting in the channel (not yet received by the Run loop)
	for i := 1; i <= 3; i++ {
		ingestBuffer.PutMessage(types.MessageId(i), time.Now(), i)
	}
	s.drainChannel()
	if err := ingestBuffer.Save(); err != nil {
		t.Fatal(err)
	}
	if len(writer.records) != 3 {
		t.Fatalf("expected 3 records saved, got %d", len(writer.records))
	}

	// end to end: messages put just before the stream stops are saved
	writer = &fakeWriter{}
	s = newRunningStream(t, writer)
	for i := 0; i < 100; i++ {
		if _, err := s.PutMessage(nil, map[string]interface{}{"n": i}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if len(writer.records) != 100 {
		t.Fatalf("expected 100 records saved, got %d", len(writer.records))
	}
}

// newRequestCtx returns a request context usable outside of a fasthttp server.
func newRequestCtx() *fasthttp.RequestCtx {
	ctx := &fasthttp.RequestCtx{}
	ctx.Init(&fasthttp.Request{}, nil, nil)
	return ctx
}
