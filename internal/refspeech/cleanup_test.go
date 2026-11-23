package refspeech

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yh2237/UtauIDX/encoder"
	"github.com/yh2237/UtauIDX/index"
)

func testIndex(model string, frames, dims int) *index.Index {
	const apBands = 1
	return &index.Index{
		Header: index.Header{
			Version: index.Version, SampleRate: 16000, APBands: apBands, FFTSize: 512, Frames: frames,
			Channels: []index.ChannelMeta{{ID: model, Dims: dims}},
		},
		Channels:     map[string][]float32{model: make([]float32, frames*dims)},
		Spectrum:     make([]float32, frames*index.SubFrames*index.SpectrumDims),
		Aperiodicity: make([]float32, frames*index.SubFrames*apBands),
		F0:           make([]float32, frames*index.SubFrames),
		FileID:       make([]int32, frames), Position: make([]int32, frames), GlitchPenalty: make([]float32, frames),
	}
}

func writeIndex(t *testing.T, dir, name, model string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := testIndex(model, 2, 4).Save(path); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCleanupRemovesUnusableIndexes(t *testing.T) {
	dir := t.TempDir()
	current := writeIndex(t, dir, "current.utsp", encoder.IDWavLMBasePlusL6)
	otherModel := writeIndex(t, dir, "other-model.utsp", encoder.IDWhisperSmallL12PosInv)
	oldVersion := filepath.Join(dir, "legacy.idx")
	if err := os.WriteFile(oldVersion, []byte("UTRM"), 0o644); err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(dir, "broken.utsp")
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
	if result.Indexes != 2 {
		t.Fatalf("removed %d indexes, want 2", result.Indexes)
	}
	for _, path := range []string{current, otherModel, keep} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s should remain: %v", filepath.Base(path), err)
		}
	}
	for _, path := range []string{oldVersion, broken} {
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
	fresh := filepath.Join(parts, "abce.part")
	stale := []string{filepath.Join(parts, "old.part"), filepath.Join(parts, "old.world"), filepath.Join(parts, "old.chan")}
	for _, path := range append([]string{fresh}, stale...) {
		if err := os.WriteFile(path, []byte("part"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-partRetention - 24*time.Hour)
	for _, path := range stale {
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}

	result, err := Cleanup(dir)
	if err != nil {
		t.Fatal(err)
	}
	if result.Parts != len(stale) {
		t.Fatalf("removed %d parts, want %d", result.Parts, len(stale))
	}
	for _, path := range stale {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("stale part %s should be removed", path)
		}
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("the fresh part should remain: %v", err)
	}
}

func TestCleanupRemovesStaleBabble(t *testing.T) {
	dir := t.TempDir()
	babble := filepath.Join(dir, "babble")
	stale, fresh := filepath.Join(babble, "stale"), filepath.Join(babble, "fresh")
	for _, path := range []string{stale, fresh} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, babbleSummaryName), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-partRetention - time.Hour)
	path := filepath.Join(stale, babbleSummaryName)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	result, err := Cleanup(dir)
	if err != nil || result.Parts != 1 || result.Bytes != 2 {
		t.Fatalf("cleanup = %+v, %v", result, err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale babble directory: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("fresh babble directory: %v", err)
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
