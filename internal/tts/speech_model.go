package tts

import (
	"fmt"
	"math"
	"strings"
	"utautts/internal/frontend"
	"utautts/internal/prosody"
	"utautts/internal/render"
)

func configureSpeechModel(cfg *Config, language string) error {
	if cfg.SpeechModel == nil && cfg.SpeechModelPath != "" {
		model, err := prosody.LoadSpeechModel(cfg.SpeechModelPath)
		if err != nil {
			return fmt.Errorf("load speech model: %w", err)
		}
		cfg.SpeechModel = model
	}
	if cfg.SpeechModel != nil {
		if err := cfg.SpeechModel.Validate(); err != nil {
			return err
		}
		if cfg.SpeechModel.Language != language {
			return fmt.Errorf("speech model language %s does not match %s", cfg.SpeechModel.Language, language)
		}
	}
	return nil
}

func speechDurationsForConfig(cfg Config, morae []frontend.Mora) [][]float64 {
	durations := speechPhoneDurations(morae, cfg.MoraDurationMS)
	if cfg.SpeechModel == nil || cfg.ProsodyPitchOnly {
		return durations
	}
	features := prosody.SpeechPhoneFeatures(morae)
	for i, m := range morae {
		for j, phone := range m.Phones {
			if !cfg.SpeechModel.Covers(phone) || m.Pause {
				continue
			}
			logRatio := prosody.SpeechLinear(cfg.SpeechModel.Duration, features[i][j])
			factor := math.Exp(math.Max(math.Log(.5), math.Min(math.Log(2), logRatio)))
			durations[i][j] = math.Max(8, math.Min(500, durations[i][j]*factor))
		}
	}
	return durations
}

func learnedSpeechEnergy(cfg Config, morae []frontend.Mora, predictions []prosody.Prediction) {
	if cfg.SpeechModel == nil || len(cfg.SpeechModel.Energy) == 0 || cfg.ProsodyPitchOnly {
		return
	}
	features := prosody.SpeechPhoneFeatures(morae)
	for i, m := range morae {
		total, count := 0.0, 0
		for j, phone := range m.Phones {
			if cfg.SpeechModel.Covers(phone) {
				total += prosody.SpeechLinear(cfg.SpeechModel.Energy, features[i][j])
				count++
			}
		}
		if count > 0 {
			predictions[i].EnergyFactor = math.Exp(math.Max(math.Log(.7), math.Min(math.Log(1.3), total/float64(count))))
		}
	}
}

func learnedSpeechCurve(cfg Config, morae []frontend.Mora, timings []prosody.MoraTiming, base *render.PitchCurve) *render.PitchCurve {
	if cfg.SpeechModel == nil || len(cfg.SpeechModel.Pitch) != 3 || base == nil || len(timings) != len(morae) {
		return base
	}
	curve := &render.PitchCurve{FrameMS: base.FrameMS, Cents: append([]float64(nil), base.Cents...)}
	features := prosody.SpeechPhoneFeatures(morae)
	durations := speechDurationsForConfig(cfg, morae)
	for i, m := range morae {
		if m.Pause {
			continue
		}
		total := 0.0
		for _, d := range durations[i] {
			total += d
		}
		if total <= 0 {
			continue
		}
		start := timings[i].StartMS
		for j, phone := range m.Phones {
			length := durations[i][j] * timings[i].DurationMS / total
			end := start + length
			if cfg.SpeechModel.PitchPhoneCounts[strings.ToLower(phone.Symbol)] >= 5 && length > 0 {
				knots := [3]float64{}
				for k := range knots {
					knots[k] = prosody.SpeechLinear(cfg.SpeechModel.Pitch[k], features[i][j])
				}
				for frame := max(0, int(math.Ceil(start/curve.FrameMS))); frame < len(curve.Cents) && float64(frame)*curve.FrameMS < end; frame++ {
					x := (float64(frame)*curve.FrameMS - start) / length * 2
					left := min(1, int(x))
					fraction := x - float64(left)
					curve.Cents[frame] = math.Max(-300, math.Min(300, knots[left]*(1-fraction)+knots[left+1]*fraction))
				}
			}
			start = end
		}
	}
	return render.ConstrainPitchCurve(curve, 16, 7)
}
