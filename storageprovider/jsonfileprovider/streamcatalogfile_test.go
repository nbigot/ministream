package jsonfileprovider

import (
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestCreateEmptyCatalogFile(t *testing.T) {
	tmpDir := t.TempDir()
	filepath := filepath.Join(tmpDir, "test_catalog.json")
	logger := zap.NewExample()
	s := NewStreamCatalogFile(logger, tmpDir, filepath)
	if err := s.CreateEmptyCatalogFile(); err != nil {
		t.Fatalf("cound not create catalog file")
	}
}
