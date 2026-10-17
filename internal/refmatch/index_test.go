package refmatch

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/yh2237/UtauTTS/internal/audio"
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

type fakeWorld struct{}

func (fakeWorld) Analyze(samples []float64, rate int, frameMS, threshold float64) ([]float64, []float64, []float64, int, error) {
	const fft = 1024
	frames := int(1000*float64(len(samples))/float64(rate)/frameMS) + 1
	f0 := make([]float64, frames)
	spectrum := make([]float64, frames*(fft/2+1))
	aperiodicity := make([]float64, frames*(fft/2+1))
	for i := range spectrum {
		spectrum[i], aperiodicity[i] = 1+float64(i%7), 0.1
	}
	return f0, spectrum, aperiodicity, fft, nil
}

func (fakeWorld) Synthesize([]float64, []float64, []float64, int, int, float64) ([]float64, error) {
	return nil, nil
}

func TestBuildIndexSkipsUnreadableWAV(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.wav")
	pcm := &audio.PCM{SampleRate: 16000, Channels: 1, Data: make([]int16, 8000)}
	for i := range pcm.Data {
		pcm.Data[i] = int16(8000 * math.Sin(float64(i)*0.1))
	}
	if err := audio.WriteWav(good, pcm); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(dir, "bad.wav")
	if err := os.WriteFile(bad, []byte("RIFF\x00\x00\x00\x00WAVEnot a pcm file"), 0o644); err != nil {
		t.Fatal(err)
	}
	features := func(samples []float64, rate int) ([]float32, int, int, error) {
		frames := len(samples) * 50 / rate
		values := make([]float32, frames*3)
		for i := range values {
			values[i] = float32(i%3 + 1)
		}
		return values, frames, 3, nil
	}
	index, err := BuildIndex([]string{bad, good}, "test", fakeWorld{}, features, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(index.Files, []string{good}) || index.Frames == 0 {
		t.Fatalf("files %v, frames %d", index.Files, index.Frames)
	}
}
