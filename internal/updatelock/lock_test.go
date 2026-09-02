package updatelock

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestWriteRejectsExistingLock(t *testing.T) {
	target := filepath.Join(t.TempDir(), "UtauTTS")
	defer Remove(target)
	if err := Write(target, "v1.2.3", 5678); err != nil {
		t.Fatal(err)
	}
	if err := Write(target, "v1.2.4", 9012); err == nil {
		t.Fatal("second updater acquired an existing lock")
	}
}

func TestWriteRejectsExistingFallbackLock(t *testing.T) {
	target := filepath.Join(t.TempDir(), "UtauTTS")
	defer Remove(target)
	if err := WriteFallback(target, "v1.2.3", 5678); err != nil {
		t.Fatal(err)
	}
	if err := Write(target, "v1.2.4", 9012); err == nil {
		t.Fatal("updater ignored an existing fallback lock")
	}
}

func TestAcquireReturnsHandoffToken(t *testing.T) {
	target := filepath.Join(t.TempDir(), "UtauTTS")
	defer Remove(target)
	token, err := Acquire(target, "v1.2.3", 5678)
	if err != nil {
		t.Fatal(err)
	}
	if token == "" {
		t.Fatal("Acquire returned an empty handoff token")
	}
	state, err := Read(target)
	if err != nil {
		t.Fatal(err)
	}
	if state.Token != token || state.UpdaterPID != 5678 {
		t.Fatalf("state = %+v, token = %q", state, token)
	}
}

func TestWriteWithTokenClaimsPendingLock(t *testing.T) {
	target := filepath.Join(t.TempDir(), "UtauTTS")
	path, err := Path(target)
	if err != nil {
		t.Fatal(err)
	}
	pending := State{Version: "v1.2.4", StartedAt: time.Now().UTC(), Token: "pending-token"}
	data, err := json.Marshal(pending)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteWithToken(target, "v1.2.4", 9012, "pending-token"); err != nil {
		t.Fatal(err)
	}
	state, err := Read(target)
	if err != nil {
		t.Fatal(err)
	}
	if state.UpdaterPID != 9012 || state.Token != "pending-token" {
		t.Fatalf("state = %+v", state)
	}
	if err := WriteWithToken(target, "v1.2.4", 3456, "pending-token"); err != nil {
		t.Fatal(err)
	}
	state, err = Read(target)
	if err != nil {
		t.Fatal(err)
	}
	if state.UpdaterPID != 3456 {
		t.Fatalf("lock handoff state = %+v", state)
	}
}

func TestReadPrefersNewestLock(t *testing.T) {
	target := filepath.Join(t.TempDir(), "UtauTTS")
	paths, err := Paths(target)
	if err != nil {
		t.Fatal(err)
	}
	old := State{Version: "old", StartedAt: time.Now().Add(-time.Minute)}
	newState := State{Version: "new", StartedAt: time.Now()}
	for path, state := range map[string]State{paths[0]: old, paths[1]: newState} {
		data, marshalErr := json.Marshal(state)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if writeErr := os.WriteFile(path, data, 0o644); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	defer Remove(target)
	state, err := Read(target)
	if err != nil {
		t.Fatal(err)
	}
	if state.Version != "new" {
		t.Fatalf("Read returned stale lock: %+v", state)
	}
}
