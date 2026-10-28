package refspeech

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yh2237/UtauIDX/refmatch"
)

// writeIndexは、指定の版とモデルで最小の索引を書き出す。
func writeIndex(t *testing.T, dir, name string, version int, model string) string {
	t.Helper()
	const frames, dims, apBands = 2, 4, 5
	index := &refmatch.Index{
		Header: refmatch.Header{
			Version: version, Model: model, SampleRate: 44100, APBands: apBands,
			FFTSize: 1024, Dims: dims, Frames: frames,
		},
		Features:     make([]float32, frames*dims),
		Spectrum:     make([]float32, frames*4*60),
		Aperiodicity: make([]float32, frames*4*apBands),
		F0:           make([]float32, frames*4),
		File:         make([]int32, frames),
	}
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := index.Save(path); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCleanupRemovesUnusableIndexes(t *testing.T) {
	dir := t.TempDir()
	current := writeIndex(t, dir, "current.idx", refmatch.Version(), FeatureModel)
	compatible := writeIndex(t, dir, "v6.idx", 6, FeatureModel)
	oldVersion := writeIndex(t, dir, "old-version.idx", 5, FeatureModel)
	otherModel := writeIndex(t, dir, "other-model.idx", refmatch.Version(), "hubert-base-layer6")
	broken := filepath.Join(dir, "broken.idx")
	if err := os.WriteFile(broken, []byte("not an index"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 索引ではないファイルは触らない。
	keep := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(keep, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Cleanup(dir)
	if err != nil {
		t.Fatal(err)
	}
	if result.Indexes != 3 {
		t.Fatalf("removed %d indexes, want 3", result.Indexes)
	}
	for _, path := range []string{current, compatible, keep} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s should remain: %v", filepath.Base(path), err)
		}
	}
	for _, path := range []string{oldVersion, otherModel, broken} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s should be removed", filepath.Base(path))
		}
	}
}

func TestCleanupRemovesUnusedParts(t *testing.T) {
	dir := t.TempDir()
	parts := filepath.Join(dir, "parts", "ab")
	if err := os.MkdirAll(parts, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(parts, "abcd.part")
	fresh := filepath.Join(parts, "abce.part")
	for _, path := range []string{stale, fresh} {
		if err := os.WriteFile(path, []byte("part"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-partRetention - 24*time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}

	result, err := Cleanup(dir)
	if err != nil {
		t.Fatal(err)
	}
	if result.Parts != 1 {
		t.Fatalf("removed %d parts, want 1", result.Parts)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("the stale part should be removed")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("the fresh part should remain: %v", err)
	}
}

func TestCleanupMissingDirectory(t *testing.T) {
	result, err := Cleanup(filepath.Join(t.TempDir(), "absent"))
	if err != nil {
		t.Fatalf("a missing directory is not an error: %v", err)
	}
	if result.Indexes != 0 || result.Parts != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestCleanupSummary(t *testing.T) {
	if got := (CleanupResult{}).Summary(); got != "" {
		t.Fatalf("empty result should have no summary: %q", got)
	}
	got := (CleanupResult{Indexes: 2, Parts: 3, Bytes: 1 << 20}).Summary()
	if got == "" {
		t.Fatal("a non-empty result should have a summary")
	}
}
