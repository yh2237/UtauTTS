package worldrender

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"

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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	const headerSize = 44
	dataSize := len(samples) * 2
	buffer := make([]byte, headerSize+dataSize)
	copy(buffer[0:], "RIFF")
	binary.LittleEndian.PutUint32(buffer[4:], uint32(36+dataSize))
	copy(buffer[8:], "WAVE")
	copy(buffer[12:], "fmt ")
	binary.LittleEndian.PutUint32(buffer[16:], 16)
	binary.LittleEndian.PutUint16(buffer[20:], 1)
	binary.LittleEndian.PutUint16(buffer[22:], 1)
	binary.LittleEndian.PutUint32(buffer[24:], uint32(sampleRate))
	binary.LittleEndian.PutUint32(buffer[28:], uint32(sampleRate*2))
	binary.LittleEndian.PutUint16(buffer[32:], 2)
	binary.LittleEndian.PutUint16(buffer[34:], 16)
	copy(buffer[36:], "data")
	binary.LittleEndian.PutUint32(buffer[40:], uint32(dataSize))
	for index, sample := range samples {
		if !isFinite32(sample) {
			sample = 0
		}
		sample = max(-1, min(1, sample*scale))
		value := int16(math.RoundToEven(float64(sample * 32767)))
		binary.LittleEndian.PutUint16(buffer[headerSize+index*2:], uint16(value))
	}
	return os.WriteFile(path, buffer, 0o644)
}

func isFinite32(value float32) bool {
	return !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0)
}
