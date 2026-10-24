package world

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yh2237/UtauTTS/internal/audio"
	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/render/base"
)

func TestNormalizeWorldSourcesWritesOncePerSource(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.wav")
	if err := audio.WriteWav(source, &audio.PCM{SampleRate: 16000, Channels: 1, Data: make([]int16, 1600)}); err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	synthesisPlan := &plan.Plan{Units: []plan.Unit{{Source: source}, {Source: source}, {Source: "missing", Silent: true}}}
	cache := base.NewSourceCache()
	normalized, err := normalizeWorldSources(synthesisPlan, &cache, 44100, output)
	if err != nil {
		t.Fatal(err)
	}
	files, err := os.ReadDir(output)
	if err != nil || len(files) != 1 || len(normalized) != 1 {
		t.Fatalf("files=%v, normalized=%v, err=%v", files, normalized, err)
	}
	pcm, err := audio.ReadWav(normalized[source])
	if err != nil || pcm.SampleRate != 44100 || len(pcm.Data) != 4410 {
		t.Fatalf("pcm=%v, err=%v", pcm, err)
	}
	unchanged, err := normalizeWorldSources(synthesisPlan, &cache, 16000, t.TempDir())
	if err != nil || len(unchanged) != 0 {
		t.Fatalf("same-rate sources=%v, err=%v", unchanged, err)
	}
}
