package refmatch

import (
	"math"
	"path/filepath"
	"reflect"
	"testing"
)

func TestIndexSaveLoad(t *testing.T) {
	index := &Index{
		Header: Header{Version: indexVersion, Model: "test", SampleRate: 44100, FFTSize: 2048, APBands: 2, Dims: 3,
			Frames: 2, F0Median: 220, Files: []string{"a.wav"}},
		Features:     []float32{0.6, 0.8, 0, -1, 0, 0},
		Spectrum:     make([]float32, 2*subFrames*spectrumDims),
		Aperiodicity: make([]float32, 2*subFrames*2),
		F0:           []float32{0, 100, 200, 300, 400, 500, 600, 700},
		File:         []int32{0, 0},
	}
	for i := range index.Spectrum {
		index.Spectrum[i] = float32(i)
	}
	path := filepath.Join(t.TempDir(), "index.bin")
	if err := index.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadIndex(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.Header, index.Header) || !reflect.DeepEqual(loaded.Spectrum, index.Spectrum) ||
		!reflect.DeepEqual(loaded.F0, index.F0) || !reflect.DeepEqual(loaded.File, index.File) {
		t.Fatal("index changed after save and load")
	}
	for i, v := range loaded.Features {
		if math.Abs(float64(v-index.Features[i])) > 1e-3 {
			t.Fatalf("feature %d = %g, want %g", i, v, index.Features[i])
		}
	}
}

func TestHalfFloat(t *testing.T) {
	for _, v := range []float32{0, 1, -1, 0.5, 65504, 1e-5, -3.14159, 1e-8} {
		got := halfToFloat32(float32ToHalf(v))
		if math.Abs(float64(got-v)) > math.Max(1e-3*math.Abs(float64(v)), 6e-8) {
			t.Errorf("half(%g) = %g", v, got)
		}
	}
}
