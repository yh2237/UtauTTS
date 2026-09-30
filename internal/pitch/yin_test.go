package pitch

import (
	"math"
	"testing"
)

func TestEstimateMedianSine(t *testing.T) {
	const sampleRate = 16000
	wave := make([]float64, sampleRate/2)
	for i := range wave {
		wave[i] = 0.2 * math.Sin(2*math.Pi*220*float64(i)/sampleRate)
	}
	got := EstimateMedian(wave, sampleRate)
	if math.Abs(got-220) > 3 {
		t.Fatalf("EstimateMedian() = %.2f, want about 220", got)
	}
}

func TestEstimateRejectsDCSignal(t *testing.T) {
	wave := make([]float64, 640)
	for i := range wave {
		wave[i] = 0.3
	}
	if got := Estimate(wave, 16000); got != 0 {
		t.Fatalf("Estimate() = %.2f for a DC signal, want 0", got)
	}
}

func TestDetectorReuseAcrossRatesAndLengths(t *testing.T) {
	var detector Detector
	for _, rate := range []int{48000, 8000, 16000, 8000} {
		for _, milliseconds := range []int{180, 40, 500, 20} {
			wave := make([]float64, rate*milliseconds/1000)
			for i := range wave {
				wave[i] = .2*math.Sin(2*math.Pi*220*float64(i)/float64(rate)) + .03
			}
			original := append([]float64(nil), wave...)
			if hz := detector.EstimateMedian(wave, rate); math.Abs(hz-220) > 3 {
				t.Fatalf("rate=%d length=%d pitch=%g", rate, milliseconds, hz)
			}
			for i := range wave {
				if wave[i] != original[i] {
					t.Fatal("input was modified")
				}
			}
		}
	}
	for _, rate := range []int{0, -16000, 1} {
		if detector.EstimateMedian([]float64{.2, -.2}, rate) != 0 {
			t.Fatal("invalid rate or short input was voiced")
		}
	}
	if detector.EstimateMedian(make([]float64, 8000), 16000) != 0 {
		t.Fatal("silence retained previous pitch")
	}
}
