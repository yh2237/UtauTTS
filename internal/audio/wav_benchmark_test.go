package audio

import (
	"fmt"
	"math"
	"path/filepath"
	"testing"
)

var benchmarkPCM *PCM
var benchmarkWAV []byte

func benchmarkAudio(rate, channels int) *PCM {
	data := make([]int16, rate*channels)
	for frame := 0; frame < rate; frame++ {
		for channel := 0; channel < channels; channel++ {
			data[frame*channels+channel] = int16(12000 * math.Sin(2*math.Pi*220*float64(frame)/float64(rate)))
		}
	}
	return &PCM{SampleRate: rate, Channels: channels, Data: data}
}

func BenchmarkReadWav(b *testing.B) {
	for _, channels := range []int{1, 2} {
		b.Run(fmt.Sprintf("48000Hz/%dch", channels), func(b *testing.B) {
			pcm := benchmarkAudio(48000, channels)
			path := filepath.Join(b.TempDir(), "source.wav")
			if err := WriteWav(path, pcm); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.SetBytes(int64(44 + len(pcm.Data)*2))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var err error
				benchmarkPCM, err = ReadWav(path)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkPCMToWavBytes(b *testing.B) {
	pcm := benchmarkAudio(48000, 1)
	b.ReportAllocs()
	b.SetBytes(int64(44 + len(pcm.Data)*2))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkWAV = PCMToWavBytes(pcm)
	}
}

func BenchmarkWriteWav(b *testing.B) {
	pcm := benchmarkAudio(48000, 1)
	path := filepath.Join(b.TempDir(), "output.wav")
	b.ReportAllocs()
	b.SetBytes(int64(44 + len(pcm.Data)*2))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := WriteWav(path, pcm); err != nil {
			b.Fatal(err)
		}
	}
}
