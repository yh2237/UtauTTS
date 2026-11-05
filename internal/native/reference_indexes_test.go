package native

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yh2237/UtauIDX/refmatch"
	"github.com/yh2237/UtauTTS/internal/refspeech"
)

func TestReferenceIndexCommands(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("LOCALAPPDATA", cache)
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("HOME", cache)
	dir, err := refspeech.DefaultCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	// A minimal index that passes the current on-disk validation; the test
	// only exercises listing and deleting, not matching.
	const spectrumDims = 60
	index := &refmatch.Index{
		Header: refmatch.Header{
			Version: refmatch.Version(), Model: "dummy", Source: filepath.Join(cache, "voicebank"), CacheKey: "dummy",
			SampleRate: 16000, FFTSize: 2, APBands: 1, Dims: 1, Frames: 1,
		},
		Features:     []float32{0},
		Spectrum:     make([]float32, refmatch.SubFrames()*spectrumDims),
		Aperiodicity: make([]float32, refmatch.SubFrames()),
		F0:           make([]float32, refmatch.SubFrames()),
		File:         []int32{0},
	}
	if err := index.Save(filepath.Join(dir, "dummy.idx")); err != nil {
		t.Fatal(err)
	}
	engine := &Engine{}
	response, err := engine.Call("listReferenceIndexes", nil)
	if err != nil {
		t.Fatal(err)
	}
	var listed struct {
		Indexes []refspeech.IndexInfo `json:"indexes"`
	}
	if err := json.Unmarshal(response, &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Indexes) != 1 || listed.Indexes[0].Voicebank != "voicebank" || listed.Indexes[0].Name != "dummy.idx" {
		t.Fatalf("list=%s", response)
	}
	response, err = engine.Call("deleteReferenceIndex", []byte(`{"name":"dummy.idx"}`))
	if err != nil || string(response) != `{"deleted":true}` {
		t.Fatalf("delete=%s err=%v", response, err)
	}
	response, err = engine.Call("listReferenceIndexes", nil)
	if err != nil || string(response) != `{"indexes":[]}` {
		t.Fatalf("empty list=%s err=%v", response, err)
	}
	if _, err := engine.Call("deleteReferenceIndex", []byte(`{"name":"../outside.idx"}`)); err == nil {
		t.Fatal("accepted path traversal")
	}
}
