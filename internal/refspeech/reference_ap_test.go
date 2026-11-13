package refspeech

import (
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/yh2237/UtauIDX/refmatch"
)

func TestSpeakAudioRejectsMismatchedTargetPhonemes(t *testing.T) {
	index := &refmatch.Index{Header: refmatch.Header{Frames: 2, Dims: 1, SampleRate: 16000, FFTSize: 512, APBands: 1},
		Features: make([]float32, 2), Cepstrum: make([]float32, 2*refmatch.CepstrumDims), Spectrum: make([]float32, 8*60),
		Aperiodicity: make([]float32, 8), F0: make([]float32, 8), File: make([]int32, 2)}
	wave := make([]float64, 960)
	for i := range wave {
		wave[i] = 0.1
	}
	_, err := (&Engine{}).SpeakAudio(index, &referenceAPWorld{}, wave, 24000,
		AudioOptions{FeatureMode: refmatch.FeatureCepstrum, TargetPhonemes: []int32{-1}})
	if err == nil || !strings.Contains(err.Error(), "phoneme") {
		t.Fatalf("target not validated: %v", err)
	}
}

func TestReferenceAPRows(t *testing.T) {
	// 8kHz/FFT16では低域の先頭4ビンだけを使う。高域のAPは判定へ混ぜない。
	ap := make([]float64, 18)
	for i := range ap {
		ap[i] = 1
	}
	for i := range 4 {
		ap[i] = 0
		ap[9+i] = 0.8
	}
	got, err := referenceAPRows(ap, 8000, 16, 20, 6)
	if err != nil {
		t.Fatal(err)
	}
	want := []float32{0, 0.2, 0.4, 0.6, 0.8, 0.8}
	for i := range want {
		if math.Abs(float64(got[i]-want[i])) > 1e-6 {
			t.Fatalf("AP = %v want %v", got, want)
		}
	}
	for _, bad := range [][]float64{nil, {1}, {math.NaN(), 1, 1, 1, 1, 1, 1, 1, 1}} {
		if _, err := referenceAPRows(bad, 8000, 16, 5, 1); err == nil {
			t.Fatal("accepted invalid AP")
		}
	}
}

type referenceAPWorld struct {
	cepstrumWorld
	audioTestWorld
}

func (w *referenceAPWorld) Analyze(samples []float64, rate int, ms, threshold float64) ([]float64, []float64, []float64, int, error) {
	f0, sp, _, fft, err := w.cepstrumWorld.Analyze(samples, rate, ms, threshold)
	ap := make([]float64, len(sp))
	bins := fft/2 + 1
	for i := range ap {
		ap[i] = 0.1
		if i/bins == 3 {
			ap[i] = 1
		}
	}
	return f0, sp, ap, fft, err
}

func (w *referenceAPWorld) Synthesize(f0, sp, ap []float64, fft, rate int, ms float64) ([]float64, error) {
	return w.audioTestWorld.Synthesize(f0, sp, ap, fft, rate, ms)
}

func TestSpeakAudioPassesAPToSynthesis(t *testing.T) {
	index := &refmatch.Index{Header: refmatch.Header{Frames: 2, Dims: 1, SampleRate: 16000, FFTSize: 512, APBands: 1, F0Median: 200}, Features: []float32{1, 1}, Spectrum: make([]float32, 8*60), Aperiodicity: make([]float32, 8), F0: []float32{200, 200, 200, 200, 200, 200, 200, 200}, File: []int32{0, 0}}
	world := &referenceAPWorld{}
	wave := make([]float64, 960)
	for i := range wave {
		wave[i] = 0.1
	}
	engine := &Engine{}
	for _, enabled := range []bool{false, true} {
		speech, err := engine.SpeakAudio(index, world, wave, 24000, AudioOptions{FeatureMode: refmatch.FeatureCepstrum, UseReferenceAP: enabled})
		if err != nil {
			t.Fatal(err)
		}
		if (world.f0[3] == 0) != enabled || !slices.Equal(world.f0, speech.F0) {
			t.Fatalf("AP enabled=%v, F0=%v", enabled, world.f0)
		}
	}
	if world.analyses != 2 || world.rate != 16000 || world.period != 5 {
		t.Fatalf("unexpected analyses: %+v", world.cepstrumWorld)
	}
}
