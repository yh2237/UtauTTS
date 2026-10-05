package acoustic

import (
	"math"

	"github.com/yh2237/audiodsp/spectrum"

	"utautts/internal/audio"
	"utautts/internal/pitch"
)

type Frame struct {
	Valid      bool      `json:"valid"`
	RMSDB      float64   `json:"rms_db"`
	F0Hz       float64   `json:"f0_hz"`
	SpectrumDB []float64 `json:"spectrum_db"`
}

func AnalyzeFrame(values []float64, sampleRate, bands int, normalizeSpectrum bool) Frame {
	rms := RMS(values)
	result := Frame{RMSDB: DB(rms)}
	if len(values) < 32 || sampleRate <= 0 || bands < 2 {
		return result
	}
	result.Valid = rms >= 1e-5
	result.F0Hz = pitch.EstimateMedian(values, sampleRate)
	result.SpectrumDB = LogSpectrum(values, sampleRate, bands, 100, math.Min(8000, float64(sampleRate)*0.45))
	if normalizeSpectrum && len(result.SpectrumDB) > 0 {
		mean := 0.0
		for _, value := range result.SpectrumDB {
			mean += value
		}
		mean /= float64(len(result.SpectrumDB))
		for index := range result.SpectrumDB {
			result.SpectrumDB[index] -= mean
		}
	}
	return result
}

func Mono(pcm *audio.PCM) []float64 {
	frames := len(pcm.Data) / pcm.Channels
	result := make([]float64, frames)
	for frame := range result {
		for channel := 0; channel < pcm.Channels; channel++ {
			result[frame] += float64(pcm.Data[frame*pcm.Channels+channel]) / 32768
		}
		result[frame] /= float64(pcm.Channels)
	}
	return result
}

func RMS(values []float64) float64 {
	energy := 0.0
	for _, value := range values {
		energy += value * value
	}
	if len(values) == 0 {
		return 0
	}
	return math.Sqrt(energy / float64(len(values)))
}

func DB(value float64) float64 {
	return 20 * math.Log10(max(value, 1e-7))
}

func LogSpectrum(values []float64, sampleRate, bands int, minimumHz, maximumHz float64) []float64 {
	return spectrum.Log(values, sampleRate, bands, minimumHz, maximumHz)
}

// 対数バンドの高域平均−低域平均を、スペクトル傾斜(dB)とする。
func SpectralTiltDB(spectrumDB []float64) float64 {
	if len(spectrumDB) < 2 {
		return 0
	}
	middle := len(spectrumDB) / 2
	low, high := 0.0, 0.0
	for _, value := range spectrumDB[:middle] {
		low += value
	}
	for _, value := range spectrumDB[middle:] {
		high += value
	}
	low /= float64(middle)
	high /= float64(len(spectrumDB) - middle)
	return high - low
}

func SpectralTiltDelta(left, right []float64) float64 {
	return math.Abs(SpectralTiltDB(left) - SpectralTiltDB(right))
}

func MeanSpectrumDelta(left, right []float64) float64 {
	length := min(len(left), len(right))
	if length == 0 {
		return 0
	}
	total := 0.0
	for index := 0; index < length; index++ {
		total += math.Abs(left[index] - right[index])
	}
	return total / float64(length)
}
