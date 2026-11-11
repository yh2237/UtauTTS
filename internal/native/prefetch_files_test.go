package native

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPrefetchFilesReadsAndStops(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sys.dic"), make([]byte, 3<<20), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	prefetchFiles(context.Background(), "")
	prefetchFiles(context.Background(), filepath.Join(dir, "missing"))
	prefetchFiles(context.Background(), dir)

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	prefetchFiles(canceled, dir)
}
