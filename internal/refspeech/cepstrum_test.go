package refspeech

import (
	"reflect"
	"testing"

	"github.com/yh2237/UtauIDX/refmatch"
)

type cepstrumWorld struct {
	rate     int
	period   float64
	analyses int
}

func (w *cepstrumWorld) Analyze(samples []float64, rate int, ms, threshold float64) ([]float64, []float64, []float64, int, error) {
	w.rate, w.period = rate, ms
	w.analyses++
	const fft, rows = 512, 8
	f0 := make([]float64, rows)
	spectrum := make([]float64, rows*(fft/2+1))
	for i := range f0 {
		f0[i] = 200
	}
	for i := range spectrum {
		spectrum[i] = 1 + float64(i%13)
	}
	return f0, spectrum, nil, fft, nil
}

func (*cepstrumWorld) Synthesize(f0, sp, ap []float64, fft, rate int, ms float64) ([]float64, error) {
	return []float64{0.1, -0.1}, nil
}

func TestSpeakCepstrumAudioAndTextWithoutWavLM(t *testing.T) {
	index := &refmatch.Index{
		Header:   refmatch.Header{Frames: 2, Dims: 2, SampleRate: 16000, FFTSize: 512, APBands: 1, F0Median: 200},
		Features: []float32{1, 0, 0, 1}, Spectrum: make([]float32, 2*4*60),
		Aperiodicity: make([]float32, 2*4), F0: []float32{200, 200, 200, 200, 200, 200, 200, 200}, File: []int32{0, 0},
	}
	// hubertがnilのEngineでも両方の経路を使える。
	engine := &Engine{}
	world := &cepstrumWorld{}
	audio := make([]float64, 960)
	for i := range audio {
		audio[i] = 0.1
	}
	wave, err := engine.SpeakAudio(index, world, audio, 24000, AudioOptions{FeatureMode: refmatch.FeatureCepstrum})
	if err != nil {
		t.Fatal(err)
	}
	if world.analyses != 1 || world.rate != 16000 || world.period != refmatch.WorldFrameMS() || len(wave.Audio) == 0 {
		t.Fatalf("wrong analysis: %+v", world)
	}
	ref := Reference{Audio: make([]float32, len(audio)), SampleRate: 24000, F0: []float32{200, 200, 200, 200}}
	for i := range ref.Audio {
		ref.Audio[i] = 0.1
	}
	text, err := engine.SpeakReference(index, world, ref, Options{FeatureMode: refmatch.FeatureCepstrum})
	if err != nil {
		t.Fatal(err)
	}
	if world.analyses != 2 || !reflect.DeepEqual(wave.Audio, text.Audio) {
		t.Fatal("text did not use WORLD cepstrum")
	}
	if !reflect.DeepEqual(index.Features, []float32{1, 0, 0, 1}) || index.Dims != 2 {
		t.Fatal("shared index modified")
	}
}

func TestSpeakRejectsUnknownSearchMode(t *testing.T) {
	engine := &Engine{}
	index := &refmatch.Index{}
	if _, err := engine.SpeakAudio(index, nil, nil, 16000, AudioOptions{FeatureMode: "hybrid"}); err == nil {
		t.Fatal("audio accepted unknown mode")
	}
	if _, err := engine.SpeakReference(index, nil, Reference{}, Options{FeatureMode: "hybrid"}); err == nil {
		t.Fatal("text accepted unknown mode")
	}
}
