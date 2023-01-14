package jsonfileprovider

import (
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestCreateEmptyCatalogFile(t *testing.T) {
	filepath := filepath.Join(t.TempDir(), "test_catalog.json")
	logger := zap.NewExample()
	s := NewStreamCatalogFile(logger, filepath)
	if err := s.CreateEmptyCatalogFile(); err != nil {
		t.Fatalf("cound not create catalog file")
	}
}
