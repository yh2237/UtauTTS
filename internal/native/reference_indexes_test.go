package native

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yh2237/UtauIDX/index"
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
	idx := &index.Index{
		Header: index.Header{
			Version: index.Version, Source: filepath.Join(cache, "voicebank"), CacheKey: "dummy",
			SampleRate: 16000, FFTSize: 2, APBands: 1, Frames: 1,
			Channels: []index.ChannelMeta{{ID: "dummy", Dims: 1}},
		},
		Channels:      map[string][]float32{"dummy": {0}},
		Spectrum:      make([]float32, index.SubFrames*index.SpectrumDims),
		Aperiodicity:  make([]float32, index.SubFrames),
		F0:            make([]float32, index.SubFrames),
		FileID:        []int32{0},
		Position:      []int32{0},
		GlitchPenalty: []float32{0},
	}
	if err := idx.Save(filepath.Join(dir, "dummy.utsp")); err != nil {
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
	if len(listed.Indexes) != 1 || listed.Indexes[0].Voicebank != "voicebank" || listed.Indexes[0].Name != "dummy.utsp" {
		t.Fatalf("list=%s", response)
	}
	response, err = engine.Call("deleteReferenceIndex", []byte(`{"name":"dummy.utsp"}`))
	if err != nil || string(response) != `{"deleted":true}` {
		t.Fatalf("delete=%s err=%v", response, err)
	}
	response, err = engine.Call("listReferenceIndexes", nil)
	if err != nil || string(response) != `{"indexes":[]}` {
		t.Fatalf("empty list=%s err=%v", response, err)
	}
	if _, err := engine.Call("deleteReferenceIndex", []byte(`{"name":"../outside.utsp"}`)); err == nil {
		t.Fatal("accepted path traversal")
	}
}
