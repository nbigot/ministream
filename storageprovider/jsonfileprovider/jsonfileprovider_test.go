package jsonfileprovider

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nbigot/ministream/types"
	"go.uber.org/zap"
)

type testFiles struct {
	dir, data, index, meta string
}

func newTestFiles(t *testing.T) testFiles {
	dir := t.TempDir()
	return testFiles{
		dir:   dir,
		data:  filepath.Join(dir, "data.jsonl"),
		index: filepath.Join(dir, "index.bin"),
		meta:  filepath.Join(dir, "stream.json"),
	}
}

func newTestWriter(t *testing.T, files testFiles, info *types.StreamInfo) *StreamWriterFile {
	w := NewStreamWriterFile(info, files.data, files.index, files.meta, zap.NewNop(), 0)
	if err := w.Init(); err != nil {
		t.Fatal(err)
	}
	if err := w.Open(); err != nil {
		t.Fatal(err)
	}
	return w
}

func writeRecords(t *testing.T, w *StreamWriterFile, first, last int) {
	records := []types.DeferedStreamRecord{}
	for i := first; i <= last; i++ {
		records = append(records, types.DeferedStreamRecord{
			Id:           types.MessageId(i),
			CreationDate: time.Now(),
			Msg:          map[string]interface{}{"n": i},
		})
	}
	if err := w.Write(&records); err != nil {
		t.Fatal(err)
	}
}

func TestWriterIndexIsReadableByTheIndex(t *testing.T) {
	files := newTestFiles(t)
	info := types.NewStreamInfo(uuid.New())
	w := newTestWriter(t, files, info)
	writeRecords(t, w, 1, 3)
	writeRecords(t, w, 4, 5)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	consistent, err := isIndexConsistent(files.data, files.index)
	if err != nil || !consistent {
		t.Fatalf("the index written by the writer is not consistent with the data file: %v", err)
	}

	data, err := os.ReadFile(files.data)
	if err != nil {
		t.Fatal(err)
	}
	idx := NewStreamIndex(info.UUID, files.index, zap.NewNop())
	for _, id := range []types.MessageId{1, 4, 5} {
		foundId, offset, err := idx.GetOffsetAtMessageId(id)
		if err != nil || foundId != id {
			t.Fatalf("message %d: found %d, error %v", id, foundId, err)
		}
		var record map[string]interface{}
		line := strings.SplitN(string(data[offset:]), "\n", 2)[0]
		if err := json.Unmarshal([]byte(line), &record); err != nil || record["i"] != float64(id) {
			t.Fatalf("message %d: wrong offset %d (%s)", id, offset, line)
		}
	}
	id, offset, err := idx.GetOffsetAfterLastMessage()
	if err != nil || id != 6 || offset != int64(len(data)) {
		t.Fatalf("after last message: id %d offset %d error %v", id, offset, err)
	}
}

func TestIndexOfPreviousVersionIsRebuilt(t *testing.T) {
	files := newTestFiles(t)
	info := types.NewStreamInfo(uuid.New())
	// data and index written by the previous versions of the writer (index rows of 20 bytes)
	lines := []string{
		`{"i":1,"d":"2023-01-15T10:21:00.5+01:00","m":{"v":1}}`,
		`{"i":2,"d":"2023-01-15T10:22:00.5+01:00","m":{"v":2}}`,
	}
	type previousRow struct {
		Id         types.MessageId
		BytesCount int32
		DateTime   int64
	}
	indexFile, err := os.Create(files.index)
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range lines {
		row := previousRow{types.MessageId(i + 1), int32(len(line) + 1), 1673774460}
		if err := binary.Write(indexFile, binary.LittleEndian, row); err != nil {
			t.Fatal(err)
		}
	}
	_ = indexFile.Close()
	if err := os.WriteFile(files.data, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	w := newTestWriter(t, files, info)
	writeRecords(t, w, 3, 3)
	_ = w.Close()

	if consistent, err := isIndexConsistent(files.data, files.index); err != nil || !consistent {
		t.Fatalf("the index was not rebuilt: %v", err)
	}
	idx := NewStreamIndex(info.UUID, files.index, zap.NewNop())
	if id, _, err := idx.GetOffsetAtMessageId(2); err != nil || id != 2 {
		t.Fatalf("message 2: found %d, error %v", id, err)
	}
}

func TestEmptyIndex(t *testing.T) {
	files := newTestFiles(t)
	if err := os.WriteFile(files.index, nil, 0644); err != nil {
		t.Fatal(err)
	}
	idx := NewStreamIndex(uuid.New(), files.index, zap.NewNop())
	if _, _, err := idx.GetOffsetAtMessageId(1); err == nil {
		t.Fatalf("expected an error")
	}
	if _, _, err := idx.GetOffsetAtTimestamp(&time.Time{}); err == nil {
		t.Fatalf("expected an error")
	}
	if id, offset, err := idx.GetOffsetLastMessage(); err != nil || id != 0 || offset != 0 {
		t.Fatalf("last message: id %d offset %d error %v", id, offset, err)
	}
	if id, offset, err := idx.GetOffsetAfterLastMessage(); err != nil || id != 1 || offset != 0 {
		t.Fatalf("after last message: id %d offset %d error %v", id, offset, err)
	}
}

func TestStreamInfoFileIsTruncated(t *testing.T) {
	files := newTestFiles(t)
	info := types.NewStreamInfo(uuid.New())
	info.Properties["description"] = strings.Repeat("x", 1000)
	w := newTestWriter(t, files, info)
	if err := w.SaveFileMetaInfo(); err != nil {
		t.Fatal(err)
	}
	info.SetProperties(&types.StreamProperties{"name": "short"})
	if err := w.SaveFileMetaInfo(); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()

	content, err := os.ReadFile(files.meta)
	if err != nil {
		t.Fatal(err)
	}
	var decoded types.StreamInfo
	if err := json.Unmarshal(content, &decoded); err != nil {
		t.Fatalf("invalid stream file: %v", err)
	}
}

func TestIteratorReadsIncompleteLinesLater(t *testing.T) {
	files := newTestFiles(t)
	info := types.NewStreamInfo(uuid.New())
	w := newTestWriter(t, files, info)
	writeRecords(t, w, 1, 1)
	_ = w.Close()

	idx := NewStreamIndex(info.UUID, files.index, zap.NewNop())
	h := NewStreamIteratorHandlerFile(info.UUID, uuid.New(), files.data, idx, zap.NewNop())
	if err := h.Open(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = h.Close() }()
	if err := h.Seek(&types.StreamIteratorRequest{IteratorType: "FIRST_MESSAGE"}); err != nil {
		t.Fatal(err)
	}
	if id, _, found, _, err := h.GetNextRecord(); !found || err != nil || id != 1 {
		t.Fatalf("expected the record 1: found=%v id=%d error=%v", found, id, err)
	}

	// a record being written
	data, err := os.OpenFile(files.data, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = data.Close() }()
	_, _ = data.WriteString(`{"i":2,"d":"2023-01-15T10:22:00Z",`)
	if _, _, found, _, _ := h.GetNextRecord(); found {
		t.Fatalf("an incomplete record must not be returned")
	}
	_, _ = data.WriteString(`"m":{"v":2}}` + "\n")
	id, record, found, _, err := h.GetNextRecord()
	if !found || err != nil || id != 2 {
		t.Fatalf("expected the record 2: found=%v id=%d error=%v", found, id, err)
	}
	if record.(map[string]interface{})["m"].(map[string]interface{})["v"] != float64(2) {
		t.Fatalf("unexpected record: %v", record)
	}
}
