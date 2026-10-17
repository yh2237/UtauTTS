package refspeech

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yh2237/UtauTTS/internal/refmatch"
	"github.com/yh2237/UtauTTS/internal/worldrender"
)

func TestPrepareRebuildsWhenRecordingsChange(t *testing.T) {
	bank := t.TempDir()
	recordings := []string{filepath.Join(bank, "a.wav"), filepath.Join(bank, "missing.wav")}
	if err := os.WriteFile(recordings[0], []byte("RIFF"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, key, err := bankRecordings(bank, recordings)
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{cacheDir: t.TempDir(), worlds: map[string]*worldrender.World{}, indexes: map[string]*indexEntry{}}
	current := &indexEntry{status: IndexStatus{State: IndexReady}, index: &refmatch.Index{Header: refmatch.Header{Source: key}}, done: make(chan struct{})}
	s.indexes[bank] = current
	s.Prepare(bank, recordings, "")
	if s.indexes[bank] != current {
		t.Fatal("unchanged recordings rebuilt the index")
	}
	if err := os.WriteFile(recordings[0], []byte("RIFF more"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.Prepare(bank, recordings, "")
	s.mu.Lock()
	replaced := s.indexes[bank] != current
	s.mu.Unlock()
	if !replaced {
		t.Fatal("changed recordings kept the old index")
	}
}
