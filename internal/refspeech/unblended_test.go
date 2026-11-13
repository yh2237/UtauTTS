package refspeech

import (
	"reflect"
	"slices"
	"testing"

	"github.com/yh2237/UtauIDX/refmatch"
)

type spectrumWorld struct {
	audioTestWorld
	spectrum []float64
}

func (w *spectrumWorld) Synthesize(f0, spectrum, ap []float64, fftSize, sampleRate int, frameMS float64) ([]float64, error) {
	w.spectrum = slices.Clone(spectrum)
	return []float64{0.1, -0.1}, nil
}

func TestUnblendedReachesMatcher(t *testing.T) {
	index := &refmatch.Index{
		Header:   refmatch.Header{Model: FeatureModel, Frames: 2, Dims: 2, SampleRate: 16000, FFTSize: 512, APBands: 1},
		Features: []float32{1, 0, 0.8, 0.6}, File: []int32{0, 1}, F0: make([]float32, 2*4),
		Spectrum: make([]float32, 2*4*60), Aperiodicity: make([]float32, 2*4),
	}
	for i := range index.Spectrum {
		index.Spectrum[i] = float32(i%7)*0.05 - float32(i%3)*0.02
	}
	render := func(unblended bool) []float64 {
		engine := &Engine{}
		engine.storeFeatures("unblended", []float32{1, 0, 1, 0, 1, 0}, 3, 2)
		world := &spectrumWorld{}
		reference := Reference{Key: "unblended", Audio: make([]float32, 1440), F0: []float32{0, 0, 0, 0, 0}}
		if _, err := engine.SpeakReference(index, world, reference, Options{Unblended: unblended}); err != nil {
			t.Fatal(err)
		}
		return world.spectrum
	}
	blended, unblended := render(false), render(true)
	if len(unblended) == 0 || reflect.DeepEqual(blended, unblended) {
		t.Fatal("Unblended did not change the matched spectrum")
	}
}
