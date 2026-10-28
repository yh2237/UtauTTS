package refspeech

import (
	"strings"
	"testing"

	"github.com/yh2237/UtauIDX/refmatch"
)

func TestAudioTargetPhonemesReachRequest(t *testing.T) {
	for _, mode := range []refmatch.FeatureMode{"wavlm", "cepstrum"} {
		t.Run(string(mode), func(t *testing.T) {
			dims := 1
			if mode == refmatch.FeatureCepstrum {
				dims = refmatch.CepstrumDims
			}
			index := &refmatch.Index{
				Header:   refmatch.Header{Model: FeatureModel, Frames: 8, Dims: dims, SampleRate: 16000, FFTSize: 512, APBands: 1},
				Features: make([]float32, 8*dims), Cepstrum: make([]float32, 8*refmatch.CepstrumDims),
				File: make([]int32, 8), F0: make([]float32, 32), Spectrum: make([]float32, 32*60), Aperiodicity: make([]float32, 32),
			}
			for _, target := range [][]int32{nil, {-1, -1, -1}, {-1}} {
				_, err := synthesizeAudio(index, &audioTestWorld{}, make([]float32, 3*dims), 3, make([]float64, 12), make([]float64, 12), AudioOptions{FeatureMode: mode, TargetPhonemes: target})
				if len(target) == 1 {
					if err == nil || !strings.Contains(err.Error(), "phoneme") {
						t.Fatalf("target not validated: %v", err)
					}
				} else if err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
