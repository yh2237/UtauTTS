package base

import (
	"math"
	"slices"
	"testing"

	"github.com/yh2237/UtauIDX/audio"
	"github.com/yh2237/UtauTTS/internal/plan"
)

func TestUnitPitchUsesTrimmedVowelAndPreservesSource(t *testing.T) {
	ClearWAVCache()
	defer ClearWAVCache()
	path := t.TempDir() + "/tone.wav"
	data := make([]int16, 16000)
	for index := range data {
		frequency := 440.0
		if index >= 9600 {
			frequency = 220
		}
		data[index] = int16(6000 * math.Sin(2*math.Pi*frequency*float64(index)/16000))
	}
	if err := audio.WriteWav(path, &audio.PCM{SampleRate: 16000, Channels: 1, Data: data}); err != nil {
		t.Fatal(err)
	}
	cache := NewSourceCache()
	mono, err := cache.LoadMono(path)
	if err != nil {
		t.Fatal(err)
	}
	unit := plan.Unit{Source: path, OffsetMS: 200, CutoffMS: -700, ConsonantMS: 400}
	first, err := EstimateUnitPitch(unit, mono)
	if err != nil {
		t.Fatal(err)
	}
	second, err := EstimateUnitPitch(unit, mono)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || math.Abs(first-220) > 3 {
		t.Fatalf("cached pitch = %.3f, first = %.3f, want 220", second, first)
	}
	if !slices.Equal(mono.Data, data) {
		t.Fatal("pitch measurement changed the source")
	}
}
