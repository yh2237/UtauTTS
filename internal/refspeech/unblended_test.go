package refspeech

import (
	"reflect"
	"slices"
	"testing"

	"github.com/yh2237/UtauIDX/encoder"
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
	id := encoder.IDWavLMBasePlusL6
	idx := testIndex(id, 2, 2)
	idx.Channels[id] = []float32{1, 0, 0.8, 0.6}
	idx.FileID = []int32{0, 1}
	for i := range idx.Spectrum {
		idx.Spectrum[i] = float32(i%7)*0.05 - float32(i%3)*0.02
	}
	render := func(unblended bool) []float64 {
		engine := &Engine{}
		engine.storeFeatures("unblended", map[string][]float32{id: {1, 0, 1, 0, 1, 0}}, 3, map[string]int{id: 2})
		world := &spectrumWorld{}
		reference := Reference{Key: "unblended", Audio: make([]float32, 1440), F0: []float32{0, 0, 0, 0, 0}}
		if _, err := engine.SpeakReference(idx, world, reference, Options{Unblended: unblended}); err != nil {
			t.Fatal(err)
		}
		return world.spectrum
	}
	blended, unblended := render(false), render(true)
	if len(unblended) == 0 || reflect.DeepEqual(blended, unblended) {
		t.Fatal("Unblended did not change the matched spectrum")
	}
}
