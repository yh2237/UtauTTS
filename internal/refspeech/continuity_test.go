package refspeech

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/yh2237/UtauIDX/refmatch"
)

type audioTestWorld struct {
	f0 []float64
}

func (*audioTestWorld) Analyze([]float64, int, float64, float64) ([]float64, []float64, []float64, int, error) {
	panic("unexpected Analyze")
}

func (w *audioTestWorld) Synthesize(f0, spectrum, ap []float64, fftSize, sampleRate int, frameMS float64) ([]float64, error) {
	w.f0 = slices.Clone(f0)
	return []float64{0.1, -0.1}, nil
}

func TestContinuityBonusAffectsTextSelection(t *testing.T) {
	for _, bonus := range []float64{0, 0.1} {
		t.Run(fmt.Sprintf("%g", bonus), func(t *testing.T) {
			const bankFrames = 8
			index := &refmatch.Index{
				Header:   refmatch.Header{Model: FeatureModel, Frames: bankFrames, Dims: 1, SampleRate: 16000, FFTSize: 512, APBands: 1},
				Features: make([]float32, bankFrames), File: make([]int32, bankFrames), F0: make([]float32, bankFrames*4),
				Spectrum: make([]float32, bankFrames*4*60), Aperiodicity: make([]float32, bankFrames*4),
			}
			for j := range index.Features {
				index.Features[j] = 1
			}
			var logs []string
			logf := func(s string) { logs = append(logs, s) }
			engine := &Engine{}
			engine.storeFeatures("continuity", []float32{1, 1, 1}, 3, 1)
			_, err := engine.SpeakReference(index, &audioTestWorld{}, Reference{Key: "continuity", Audio: make([]float32, 1440), F0: []float32{0, 0, 0, 0, 0}}, Options{ContinuityBonus: bonus, Logf: logf})
			if err != nil {
				t.Fatal(err)
			}
			want := "Switches=2"
			if bonus > 0 {
				want = "Switches=0"
			}
			if !strings.Contains(strings.Join(logs, "\n"), want) {
				t.Fatalf("missing %s in %v", want, logs)
			}
		})
	}
}

func TestSpeakAudioRejectsInvalidContinuityBonus(t *testing.T) {
	index := &refmatch.Index{Header: refmatch.Header{Frames: 2, Dims: 1, SampleRate: 16000, FFTSize: 512, APBands: 1},
		Features: make([]float32, 2), Cepstrum: make([]float32, 2*refmatch.CepstrumDims),
		Spectrum: make([]float32, 8*60), Aperiodicity: make([]float32, 8), F0: make([]float32, 8), File: make([]int32, 2)}
	wave := make([]float64, 960)
	for i := range wave {
		wave[i] = 0.1
	}
	_, err := (&Engine{}).SpeakAudio(index, &referenceAPWorld{}, wave, 24000,
		AudioOptions{FeatureMode: refmatch.FeatureCepstrum, ContinuityBonus: -0.1})
	if err == nil || !strings.Contains(err.Error(), "continuity bonus") {
		t.Fatalf("continuity bonus not validated: %v", err)
	}
}
