package atomicfile

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestFailedWritePreservesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "voice.wav")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	err := Write(path, func(w io.Writer) error { w.Write([]byte("partial")); return errors.New("disk failure") })
	if err == nil {
		t.Fatal("failure lost")
	}
	data, _ := os.ReadFile(path)
	entries, _ := os.ReadDir(dir)
	if string(data) != "original" || len(entries) != 1 {
		t.Fatalf("data=%q entries=%d", data, len(entries))
	}
	if err := WriteFile(path, []byte("complete")); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if string(data) != "complete" {
		t.Fatalf("replacement=%q", data)
	}
}
