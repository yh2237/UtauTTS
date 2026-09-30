package base

import (
	"math"
	"path/filepath"
	"testing"

	"utautts/internal/audio"
)

var benchmarkWave []float64
var benchmarkCachedPCM *audio.PCM

func benchmarkSource() []float64 {
	wave := make([]float64, 4800)
	for i := range wave {
		wave[i] = .25 * math.Sin(2*math.Pi*220*float64(i)/16000)
	}
	return wave
}

func BenchmarkWSOLA(b *testing.B) {
	source := benchmarkSource()
	for _, target := range []struct {
		name   string
		frames int
	}{{"compress", 3200}, {"expand", 7200}} {
		b.Run(target.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchmarkWave = StretchWSOLA(source, target.frames, 16000)
			}
		})
	}
}

func BenchmarkWSOLAAnchored(b *testing.B) {
	source := benchmarkSource()
	sourceAnchors := []int{0, 1600, 3200, 4800}
	targetAnchors := []int{0, 800, 2400, 4000}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkWave = StretchWSOLAAnchored(source, 4000, 16000, sourceAnchors, targetAnchors)
	}
}

// missはGo側のキャッシュだけを空にする。OSのファイルキャッシュは制御しない。
func BenchmarkWAVCache(b *testing.B) {
	path := filepath.Join(b.TempDir(), "source.wav")
	pcm := &audio.PCM{SampleRate: 16000, Channels: 1, Data: floatPCM(benchmarkSource())}
	if err := audio.WriteWav(path, pcm); err != nil {
		b.Fatal(err)
	}
	for _, mode := range []string{"hit", "miss"} {
		b.Run(mode, func(b *testing.B) {
			ClearWAVCache()
			b.Cleanup(ClearWAVCache)
			if _, err := loadWAVCached(path); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if mode == "miss" {
					b.StopTimer()
					ClearWAVCache()
					b.StartTimer()
				}
				var err error
				benchmarkCachedPCM, err = loadWAVCached(path)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
