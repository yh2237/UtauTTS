package updatelock

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteReadRemove(t *testing.T) {
	target := filepath.Join(t.TempDir(), "UtauTTS")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Write(target, "v1.2.3", 1234); err != nil {
		t.Fatal(err)
	}
	state, err := Read(target)
	if err != nil {
		t.Fatal(err)
	}
	if state.Version != "v1.2.3" || state.UpdaterPID != 1234 || state.StartedAt.IsZero() {
		t.Fatalf("state = %+v", state)
	}
	path, err := Path(target)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != filepath.Dir(target) {
		t.Fatalf("lock must be outside target: %s", path)
	}
	if err := Remove(target); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("lock still exists: %v", err)
	}
}

func TestReadAcceptsQtPendingLock(t *testing.T) {
	target := filepath.Join(t.TempDir(), "UtauTTS")
	path, err := Path(target)
	if err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"version":"v1.2.3","started_at":"2026-08-20T12:34:56.789Z","updater_pid":0}`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := Read(target)
	if err != nil {
		t.Fatal(err)
	}
	if state.UpdaterPID != 0 || state.StartedAt.IsZero() {
		t.Fatalf("state = %+v", state)
	}
}

func TestReadFallbackLock(t *testing.T) {
	target := filepath.Join(t.TempDir(), "UtauTTS")
	if err := WriteFallback(target, "v1.2.4", 5678); err != nil {
		t.Fatal(err)
	}
	defer Remove(target)

	state, err := Read(target)
	if err != nil {
		t.Fatal(err)
	}
	if state.Version != "v1.2.4" || state.UpdaterPID != 5678 || state.StartedAt.IsZero() {
		t.Fatalf("state = %+v", state)
	}
}
