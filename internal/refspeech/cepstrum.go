package refspeech

import (
	"fmt"

	"github.com/yh2237/UtauIDX/refmatch"
	"github.com/yh2237/gograd/dsp"
)

// analyzeCepstrumは索引と同じ周波数・5ms周期で包絡を解析する。
// WORLDの符号は標本化周波数に依存するため、先に参照波形の周波数を揃える。
func analyzeCepstrum(index *refmatch.Index, world refmatch.World, samples []float64, rate int) ([]float32, int, []float64, error) {
	features, frames, f0, _, err := analyzeCepstrumAP(index, world, samples, rate, false)
	return features, frames, f0, err
}

func analyzeCepstrumAP(index *refmatch.Index, world refmatch.World, samples []float64, rate int, useAP bool) ([]float32, int, []float64, []float32, error) {
	if rate <= 0 || index.SampleRate <= 0 || len(samples) == 0 {
		return nil, 0, nil, nil, fmt.Errorf("invalid reference audio or index sample rate")
	}
	if rate != index.SampleRate {
		wave := make([]float32, len(samples))
		for i, v := range samples {
			wave[i] = float32(v)
		}
		resampled := dsp.Resample(wave, rate, index.SampleRate)
		samples = make([]float64, len(resampled))
		for i, v := range resampled {
			samples[i] = float64(v)
		}
		rate = index.SampleRate
	}
	f0, spectrum, ap, fftSize, err := world.Analyze(samples, rate, refmatch.WorldFrameMS(), 0.85)
	if err != nil {
		return nil, 0, nil, nil, fmt.Errorf("analyze reference: %w", err)
	}
	features, frames, err := refmatch.ReferenceCepstrum(spectrum, rate, fftSize, refmatch.WorldFrameMS())
	var referenceAP []float32
	if useAP && err == nil {
		referenceAP, err = referenceAPRows(ap, rate, fftSize, refmatch.WorldFrameMS(), frames*refmatch.SubFrames())
	}
	return features, frames, f0, referenceAP, err
}
