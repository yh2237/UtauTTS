package refspeech

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/yh2237/UtauIDX/encoder"
	"github.com/yh2237/UtauIDX/index"
)

type audioTestWorld struct {
	f0 []float64
}

func (*audioTestWorld) Analyze(samples []float64, rate int, frameMS, _ float64) ([]float64, []float64, []float64, int, error) {
	const fftSize = 512
	rows := int(float64(len(samples))*1000/float64(rate)/frameMS) + 1
	spectrum := make([]float64, rows*(fftSize/2+1))
	ap := make([]float64, len(spectrum))
	for i := range spectrum {
		spectrum[i], ap[i] = 1, 0.2
	}
	return make([]float64, rows), spectrum, ap, fftSize, nil
}

func (w *audioTestWorld) Synthesize(f0, spectrum, ap []float64, fftSize, sampleRate int, frameMS float64) ([]float64, error) {
	w.f0 = slices.Clone(f0)
	return []float64{0.1, -0.1}, nil
}

func TestContinuityBonusAffectsTextSelection(t *testing.T) {
	for _, bonus := range []float64{0, 0.1} {
		t.Run(fmt.Sprintf("%g", bonus), func(t *testing.T) {
			const bankFrames = 8
			id := encoder.IDWavLMBasePlusL6
			idx := testIndex(id, bankFrames, 1)
			for j := range idx.Position {
				idx.Channels[id][j] = 1
				idx.Position[j] = int32(j)
			}
			var logs []string
			logf := func(s string) { logs = append(logs, s) }
			engine := &Engine{}
			engine.storeFeatures("continuity", map[string][]float32{id: {1, 1, 1}}, 3, map[string]int{id: 1})
			_, err := engine.SpeakReference(idx, &audioTestWorld{}, Reference{Key: "continuity", Audio: make([]float32, 1440), F0: []float32{0, 0, 0, 0, 0}}, Options{ContinuityBonus: &bonus, Logf: logf})
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
	for _, bonus := range []float64{-0.1, math.NaN(), math.Inf(1)} {
		_, err := (&Engine{}).SpeakAudio(&index.Index{}, &audioTestWorld{}, []float64{0.1}, 24000,
			AudioOptions{ContinuityBonus: &bonus})
		if err == nil || !strings.Contains(err.Error(), "continuity bonus") {
			t.Fatalf("continuity bonus %g not validated: %v", bonus, err)
		}
	}
}
