package refspeech

import (
	"fmt"
	"strings"
	"testing"

	"github.com/yh2237/UtauIDX/refmatch"
)

func TestContinuityOptionsReachBothPaths(t *testing.T) {
	for _, path := range []string{"audio", "text"} {
		for _, bonus := range []float64{0, 0.1} {
			t.Run(fmt.Sprintf("%s/%g", path, bonus), func(t *testing.T) {
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
				var err error
				if path == "audio" {
					_, err = synthesizeAudio(index, &audioTestWorld{}, []float32{1, 1, 1}, 3, make([]float64, 12), make([]float64, 12), AudioOptions{ContinuityBonus: bonus, Logf: logf})
				} else {
					engine := &Engine{}
					engine.storeFeatures("continuity", []float32{1, 1, 1}, 3, 1)
					_, err = engine.SpeakReference(index, &audioTestWorld{}, Reference{Key: "continuity", Audio: make([]float32, 1440), F0: []float32{0, 0, 0, 0, 0}}, Options{ContinuityBonus: bonus, Logf: logf})
				}
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
}
