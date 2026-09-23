package base

import (
	"math"
	"testing"

	"utautts/internal/audio"
	"utautts/internal/plan"
)

func TestUnitPitchCacheIsReusedAndClearedWithWAVCache(t *testing.T) {
	ClearWAVCache()
	defer ClearWAVCache()
	path := t.TempDir() + "/tone.wav"
	data := make([]int16, 4000)
	for index := range data {
		data[index] = int16(6000 * math.Sin(2*math.Pi*220*float64(index)/16000))
	}
	if err := audio.WriteWav(path, &audio.PCM{SampleRate: 16000, Channels: 1, Data: data}); err != nil {
		t.Fatal(err)
	}
	cache := NewSourceCache()
	mono, err := cache.LoadMono(path)
	if err != nil {
		t.Fatal(err)
	}
	unit := plan.Unit{Source: path}
	first, err := EstimateUnitPitch(unit, mono)
	if err != nil {
		t.Fatal(err)
	}
	second, err := EstimateUnitPitch(unit, mono)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || first <= 0 {
		t.Fatalf("cached pitch = %.3f, first = %.3f", second, first)
	}
	if len(globalUnitPitchCache.entries) != 1 {
		t.Fatalf("pitch cache entries = %d, want 1", len(globalUnitPitchCache.entries))
	}
	ClearWAVCache()
	if len(globalUnitPitchCache.entries) != 0 {
		t.Fatal("pitch cache was not cleared with the WAV cache")
	}
}
