package refspeech

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yh2237/UtauIDX/encoder"
	"github.com/yh2237/UtauIDX/index"
	"github.com/yh2237/UtauIDX/speechpool"
	"github.com/yh2237/UtauWLD/worldrender"
	"github.com/yh2237/audiodsp/wav"
)

func TestPrepareRebuildsWhenRecordingsChange(t *testing.T) {
	bank := t.TempDir()
	recordings := []speechpool.Entry{{Path: filepath.Join(bank, "a.wav")}, {Path: filepath.Join(bank, "missing.wav")}}
	if err := os.WriteFile(recordings[0].Path, []byte("RIFF"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, key, err := bankRecordings(bank, "", recordings)
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{cacheDir: t.TempDir(), worlds: map[string]*worldrender.World{}, indexes: map[string]*indexEntry{}}
	current := &indexEntry{status: IndexStatus{State: IndexReady}, index: &index.Index{Header: index.Header{Source: bank, CacheKey: indexCacheKey(key, nil, configuredPoolMode(), configuredReversedPool())}}, done: make(chan struct{})}
	s.indexes[indexID(bank, "", recordings)] = current
	s.Prepare(bank, "", recordings, "")
	if s.indexes[indexID(bank, "", recordings)] != current {
		t.Fatal("unchanged recordings rebuilt the index")
	}
	if err := os.WriteFile(recordings[0].Path, []byte("RIFF more"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.Prepare(bank, "", recordings, "")
	s.mu.Lock()
	replaced := s.indexes[indexID(bank, "", recordings)] != current
	s.mu.Unlock()
	if !replaced {
		t.Fatal("changed recordings kept the old index")
	}
}

func TestIndexRebuildsWhenRecordingsChange(t *testing.T) {
	bank := t.TempDir()
	recordings := []speechpool.Entry{{Path: filepath.Join(bank, "a.wav")}}
	if err := os.WriteFile(recordings[0].Path, []byte("RIFF"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, key, err := bankRecordings(bank, "", recordings)
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{cacheDir: t.TempDir(), worlds: map[string]*worldrender.World{}, indexes: map[string]*indexEntry{}}
	done := make(chan struct{})
	close(done)
	current := &indexEntry{status: IndexStatus{State: IndexReady}, index: &index.Index{Header: index.Header{Source: bank, CacheKey: indexCacheKey(key, nil, configuredPoolMode(), configuredReversedPool())}}, done: done}
	s.indexes[indexID(bank, "", recordings)] = current
	if index, err := s.Index(context.Background(), bank, "", recordings, ""); err != nil || index != current.index {
		t.Fatalf("unchanged recordings: index=%p err=%v", index, err)
	}
	if err := os.WriteFile(recordings[0].Path, []byte("RIFF more"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _ = s.Index(ctx, bank, "", recordings, "")
	s.mu.Lock()
	replaced := s.indexes[indexID(bank, "", recordings)] != current
	s.mu.Unlock()
	if !replaced {
		t.Fatal("changed recordings kept the old index")
	}
}

func TestBuildReleasesWaitersOnFailure(t *testing.T) {
	bank := t.TempDir()
	recordings := []speechpool.Entry{{Path: filepath.Join(bank, "a.wav")}}
	entry := &indexEntry{status: IndexStatus{State: IndexBuilding}, done: make(chan struct{})}
	s := &Service{cacheDir: t.TempDir(), worlds: map[string]*worldrender.World{}, indexes: map[string]*indexEntry{}}
	s.build(bank, "", recordings, filepath.Join(bank, "missing-world-engine.so"), entry)
	select {
	case <-entry.done:
	default:
		t.Fatal("build did not release the waiter")
	}
	s.mu.Lock()
	state := entry.status.State
	s.mu.Unlock()
	if state != IndexFailed {
		t.Fatalf("state = %v, want %v", state, IndexFailed)
	}
}

func TestExternalLoadOrBuildKeepsBabbleBeforeWorldAndSeparatesDir(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "cache")
	bankDir := filepath.Join(dir, "bank")
	if err := os.MkdirAll(bankDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const sr = 16000
	aWav := filepath.Join(bankDir, "a.wav")
	if err := wav.WriteFile(aWav, &wav.PCM{SampleRate: sr, Channels: 1, Data: make([]int16, sr/2)}); err != nil {
		t.Fatal(err)
	}
	entries := []speechpool.Entry{{Path: aWav, Layer: speechpool.LayerRaw}}
	_, source, err := bankRecordings(bankDir, "", entries)
	if err != nil {
		t.Fatal(err)
	}
	if babbleDirPath(cacheDir, source, false) == babbleDirPath(cacheDir, source, true) {
		t.Fatal("external babbleDir should be isolated from native babbleDir")
	}
	babbleDir := babbleDirPath(cacheDir, source, true)
	babbleWav := filepath.Join(babbleDir, "babble_000.wav")
	if err := os.MkdirAll(babbleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := wav.WriteFile(babbleWav, &wav.PCM{SampleRate: sr, Channels: 1, Data: make([]int16, sr/2)}); err != nil {
		t.Fatal(err)
	}
	summaryBytes, err := json.Marshal(speechpool.BabbleSummary{Phrases: 1, Files: []string{babbleWav}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(babbleDir, babbleSummaryName), summaryBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	store := encoder.NewExternalStore()
	reg := encoder.NewRegistry(store.Encoder(encoder.IDWavLMBasePlusL6, 768, encoder.DefaultFrameMS, encoder.ConvFrameCount))
	key := indexCacheKey(source, reg.IDs(), configuredPoolMode(), configuredReversedPool())
	eng := &Engine{external: true, registry: reg, externalStore: store, prefetched: map[string]bool{}}
	eng.ProvidePrefetched(key)
	s := &Service{cacheDir: cacheDir, engine: eng, worlds: map[string]*worldrender.World{}, indexes: map[string]*indexEntry{}}
	if _, _, err := s.loadOrBuild(bankDir, "", entries, filepath.Join(dir, "missing-world"), func(string, ...any) {}, nil); err == nil {
		t.Fatal("expected missing WORLD error")
	}
	if _, err := os.Stat(babbleDir); err != nil {
		t.Fatalf("babbleDir removed on failed build: %v", err)
	}
}

func TestConfiguredReversedPoolAndCacheKey(t *testing.T) {
	if !configuredReversedPool() {
		t.Fatal("configuredReversedPool() = false by default, want true")
	}
	bank := t.TempDir()
	entries := []speechpool.Entry{{Path: filepath.Join(bank, "a.wav")}}
	idOn := indexID(bank, "", entries)
	keyOn := indexCacheKey("src", []string{"enc"}, speechpool.PoolAuto, configuredReversedPool())

	for _, val := range []string{"0", "false", "off", "no", " FALSE "} {
		t.Setenv("UTAUTTS_REVERSED_POOL", val)
		if configuredReversedPool() {
			t.Fatalf("configuredReversedPool() = true for %q, want false", val)
		}
	}
	idOff := indexID(bank, "", entries)
	keyOff := indexCacheKey("src", []string{"enc"}, speechpool.PoolAuto, configuredReversedPool())
	if idOn == idOff || keyOn == keyOff {
		t.Fatalf("cache keys matched across reversed toggle: id=(%s,%s) key=(%s,%s)", idOn, idOff, keyOn, keyOff)
	}
}

func TestDifferentToneRecordingsUseDistinctCacheKeys(t *testing.T) {
	bank := t.TempDir()
	b3Dir, d4Dir := filepath.Join(bank, "B3"), filepath.Join(bank, "D#4")
	if err := os.MkdirAll(b3Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(d4Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	b3Wav := filepath.Join(b3Dir, "a.wav")
	d4Wav := filepath.Join(d4Dir, "a.wav")
	for _, p := range []string{b3Wav, d4Wav} {
		if err := wav.WriteFile(p, &wav.PCM{SampleRate: 16000, Channels: 1, Data: make([]int16, 8000)}); err != nil {
			t.Fatal(err)
		}
	}
	b3Entries := []speechpool.Entry{{Path: b3Wav, Layer: speechpool.LayerRaw}}
	d4Entries := []speechpool.Entry{{Path: d4Wav, Layer: speechpool.LayerRaw}}
	if indexID(bank, "", b3Entries) == indexID(bank, "", d4Entries) {
		t.Fatal("different tone recording sets produced the same indexID")
	}
	_, srcB3, err := bankRecordings(bank, "", b3Entries)
	if err != nil {
		t.Fatal(err)
	}
	_, srcD4, err := bankRecordings(bank, "", d4Entries)
	if err != nil {
		t.Fatal(err)
	}
	if srcB3 == srcD4 {
		t.Fatal("different tone recording sets produced the same source hash")
	}
	keyB3 := indexCacheKey(srcB3, []string{"enc"}, configuredPoolMode(), configuredReversedPool())
	keyD4 := indexCacheKey(srcD4, []string{"enc"}, configuredPoolMode(), configuredReversedPool())
	if keyB3 == keyD4 || babbleDirPath("/cache", srcB3, false) == babbleDirPath("/cache", srcD4, false) {
		t.Fatalf("different tone recording sets shared cache or babble keys: (%s, %s)", keyB3, keyD4)
	}
}
