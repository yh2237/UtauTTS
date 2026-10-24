//go:build !js

package tts

import (
	"fmt"
	"math"

	"github.com/yh2237/UtauTTS/internal/frontend"
)

// moraAdjustRampMSは、隣のモーラと値が違うときに、境界の前後でなめらかにつなぐ長さ。
const moraAdjustRampMS = 20.0

// referenceUnitAdjustmentsは、拡張編集のPIT（pitch_factor）とVOL（resampler_volume・energy_factor）を、
// モーラの番号ごとのセントと音量の倍率にする。参照照合の音素は休止でないモーラの順に並ぶ。
func referenceUnitAdjustments(cfg Config, morae []frontend.Mora) (map[int]float64, map[int]float64, error) {
	var units []int
	for index, mora := range morae {
		if !mora.Pause {
			units = append(units, index)
		}
	}
	cents, gain := map[int]float64{}, map[int]float64{}
	for _, override := range cfg.UnitOverrides {
		if override.Index < 0 || override.Index >= len(units) {
			return nil, nil, fmt.Errorf("unit override index %d is outside 0..%d", override.Index, len(units)-1)
		}
		mora := units[override.Index]
		if override.PitchFactor != nil {
			if *override.PitchFactor <= 0 || math.IsNaN(*override.PitchFactor) {
				return nil, nil, fmt.Errorf("unit %d pitch_factor must be positive", override.Index)
			}
			cents[mora] = 1200 * math.Log2(*override.PitchFactor)
		}
		factor := 1.0
		if override.ResamplerVolume != nil {
			if *override.ResamplerVolume < 0 || *override.ResamplerVolume > 200 {
				return nil, nil, fmt.Errorf("unit %d resampler_volume must be between 0 and 200", override.Index)
			}
			factor *= float64(*override.ResamplerVolume) / 100
		}
		if override.EnergyFactor != nil {
			if *override.EnergyFactor < 0 || math.IsNaN(*override.EnergyFactor) {
				return nil, nil, fmt.Errorf("unit %d energy_factor must not be negative", override.Index)
			}
			factor *= *override.EnergyFactor
		}
		if override.ResamplerVolume != nil || override.EnergyFactor != nil {
			gain[mora] = factor
		}
	}
	return cents, gain, nil
}

// moraEnvelopeは、最初のモーラの始まりからのミリ秒ごとに、モーラの値を返す関数を作る。
// 値のないモーラはneutral。境界の前後moraAdjustRampMS/2は、両側の値を直線でつなぐ。
func moraEnvelope(durations []float64, values map[int]float64, neutral float64) func(float64) float64 {
	if len(values) == 0 {
		return nil
	}
	starts := make([]float64, len(durations)+1)
	for i, d := range durations {
		starts[i+1] = starts[i] + d
	}
	value := func(i int) float64 {
		if v, ok := values[i]; ok {
			return v
		}
		return neutral
	}
	return func(ms float64) float64 {
		i := 0
		for i+1 < len(durations) && ms >= starts[i+1] {
			i++
		}
		current := value(i)
		half := moraAdjustRampMS / 2
		if i+1 < len(durations) && ms > starts[i+1]-half {
			t := (ms - (starts[i+1] - half)) / moraAdjustRampMS
			return current + (value(i+1)-current)*t
		}
		if i > 0 && ms < starts[i]+half {
			t := (ms - (starts[i] - half)) / moraAdjustRampMS
			return value(i-1) + (current-value(i-1))*t
		}
		return current
	}
}
