package acoustic

import (
	"math"
	"testing"
)

var benchmarkFrame Frame
var benchmarkSpectrum []float64

func benchmarkAcousticWave() []float64 {
	wave := make([]float64, 640)
	for i := range wave {
		phase := 2 * math.Pi * 220 * float64(i) / 16000
		wave[i] = .25*math.Sin(phase) + .08*math.Sin(phase*2)
	}
	return wave
}

func BenchmarkAnalyzeFrame(b *testing.B) {
	wave := benchmarkAcousticWave()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkFrame = AnalyzeFrame(wave, 16000, 24, true)
	}
}

func BenchmarkLogSpectrum(b *testing.B) {
	wave := benchmarkAcousticWave()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkSpectrum = LogSpectrum(wave, 16000, 24, 100, 7200)
	}
}
