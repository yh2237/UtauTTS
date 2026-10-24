package worldrender

import (
	"fmt"
	"math"

	"github.com/yh2237/UtauTTS/internal/audio"
)

// 音源選択と同じ読込規則にそろえる。RIFFサイズの誤りや、奇数長dataの端数は許容する。
func readPCM16(path string) (int, []float64, error) {
	pcm, err := audio.ReadWav(path)
	if err != nil {
		return 0, nil, fmt.Errorf("read WAV %s: %w", path, err)
	}
	samples := make([]float64, len(pcm.Data)/pcm.Channels)
	for frame := range samples {
		var sum float64
		for _, sample := range pcm.Data[frame*pcm.Channels : (frame+1)*pcm.Channels] {
			sum += float64(sample) / 32768
		}
		samples[frame] = sum / float64(pcm.Channels)
	}
	return pcm.SampleRate, samples, nil
}

func writePCM16(path string, sampleRate int, samples []float32) error {
	return audio.WriteWav(path, &audio.PCM{SampleRate: sampleRate, Channels: 1, Data: pcm16(samples)})
}

// pcm16は、ピークが0.98を超えるときだけ縮めて16bitにする。
func pcm16(samples []float32) []int16 {
	var peak float32
	for _, sample := range samples {
		if isFinite32(sample) && float32(math.Abs(float64(sample))) > peak {
			peak = float32(math.Abs(float64(sample)))
		}
	}
	scale := float32(1)
	if peak > 0.98 {
		scale = 0.98 / peak
	}
	data := make([]int16, len(samples))
	for index, sample := range samples {
		if !isFinite32(sample) {
			sample = 0
		}
		sample = max(-1, min(1, sample*scale))
		data[index] = int16(math.RoundToEven(float64(sample * 32767)))
	}
	return data
}

func isFinite32(value float32) bool {
	return !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0)
}
