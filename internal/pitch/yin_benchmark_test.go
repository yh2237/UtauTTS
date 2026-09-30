package pitch

import (
	"fmt"
	"math"
	"testing"
)

var benchmarkHz float64

func benchmarkPitchWave(rate, milliseconds int) []float64 {
	wave := make([]float64, rate*milliseconds/1000)
	for i := range wave {
		phase := 2 * math.Pi * 220 * float64(i) / float64(rate)
		wave[i] = .25*math.Sin(phase) + .08*math.Sin(phase*2)
	}
	return wave
}

func BenchmarkPitchEstimate(b *testing.B) {
	for _, rate := range []int{8000, 16000, 48000} {
		b.Run(fmt.Sprintf("%dHz/40ms", rate), func(b *testing.B) {
			wave := benchmarkPitchWave(rate, 40)
			if hz := Estimate(wave, rate); math.Abs(hz-220) > 5 {
				b.Fatalf("fixture pitch = %v", hz)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchmarkHz = Estimate(wave, rate)
			}
		})
	}
}

func BenchmarkPitchEstimateMedian(b *testing.B) {
	wave := benchmarkPitchWave(16000, 180)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkHz = EstimateMedian(wave, 16000)
	}
}
