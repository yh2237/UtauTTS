package refspeech

import (
	"math"
	"slices"
	"testing"

	"github.com/yh2237/UtauIDX/refmatch"
)

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

func TestSynthesizeAudioAPOverridesLevel(t *testing.T) {
	index := &refmatch.Index{Header: refmatch.Header{Frames: 1, Dims: 1, SampleRate: 16000, FFTSize: 512, APBands: 1, F0Median: 200}, Features: []float32{1}, Spectrum: make([]float32, 4*60), Aperiodicity: []float32{-40, -40, -40, -40}, F0: []float32{200, 200, 200, 200}, File: []int32{0}}
	world := &audioTestWorld{}
	speech, err := synthesizeAudioAP(index, world, []float32{1}, 1, []float64{200, 200, 0, 200}, []float64{0, 0, 0, -60}, []float32{0.1, 1, 0.1, 0.1}, AudioOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(world.f0, []float64{200, 0, 0, 200}) || !slices.Equal(speech.F0, world.f0) {
		t.Fatalf("F0 = %v", world.f0)
	}
	if speech.ReferenceVoicing.After != 2 {
		t.Fatalf("voicing = %+v", speech.ReferenceVoicing)
	}
}
