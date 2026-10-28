package refspeech

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/yh2237/UtauIDX/refmatch"
	"github.com/yh2237/UtauWLD/worldrender"
)

func TestPrepareRebuildsWhenRecordingsChange(t *testing.T) {
	bank := t.TempDir()
	recordings := []refmatch.Recording{{Path: filepath.Join(bank, "a.wav")}, {Path: filepath.Join(bank, "missing.wav")}}
	if err := os.WriteFile(recordings[0].Path, []byte("RIFF"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, key, err := bankRecordings(bank, recordings)
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{cacheDir: t.TempDir(), worlds: map[string]*worldrender.World{}, indexes: map[string]*indexEntry{}}
	current := &indexEntry{status: IndexStatus{State: IndexReady}, index: &refmatch.Index{Header: refmatch.Header{Source: key}}, done: make(chan struct{})}
	s.indexes[indexID(bank, recordings)] = current
	s.Prepare(bank, recordings, "")
	if s.indexes[indexID(bank, recordings)] != current {
		t.Fatal("unchanged recordings rebuilt the index")
	}
	if err := os.WriteFile(recordings[0].Path, []byte("RIFF more"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.Prepare(bank, recordings, "")
	s.mu.Lock()
	replaced := s.indexes[indexID(bank, recordings)] != current
	s.mu.Unlock()
	if !replaced {
		t.Fatal("changed recordings kept the old index")
	}
}

func TestIndexRebuildsWhenRecordingsChange(t *testing.T) {
	bank := t.TempDir()
	recordings := []refmatch.Recording{{Path: filepath.Join(bank, "a.wav")}}
	if err := os.WriteFile(recordings[0].Path, []byte("RIFF"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, key, err := bankRecordings(bank, recordings)
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{cacheDir: t.TempDir(), worlds: map[string]*worldrender.World{}, indexes: map[string]*indexEntry{}}
	done := make(chan struct{})
	close(done)
	current := &indexEntry{status: IndexStatus{State: IndexReady}, index: &refmatch.Index{Header: refmatch.Header{Source: key}}, done: done}
	s.indexes[indexID(bank, recordings)] = current
	if index, err := s.Index(context.Background(), bank, recordings, ""); err != nil || index != current.index {
		t.Fatalf("unchanged recordings: index=%p err=%v", index, err)
	}
	if err := os.WriteFile(recordings[0].Path, []byte("RIFF more"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _ = s.Index(ctx, bank, recordings, "")
	s.mu.Lock()
	replaced := s.indexes[indexID(bank, recordings)] != current
	s.mu.Unlock()
	if !replaced {
		t.Fatal("changed recordings kept the old index")
	}
}
