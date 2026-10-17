package tts

import (
	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/prosody"
)

// 原音alias数に依存せず、音素ごとの長さを決める。
func speechPhoneDurations(morae []frontend.Mora, base float64) [][]float64 {
	if base <= 0 {
		base = plan.DefaultMoraDurationMS
	}
	result := make([][]float64, len(morae))
	for i, m := range morae {
		if m.Pause {
			continue
		}
		result[i] = make([]float64, len(m.Phones))
		rhymeShares := frontend.MandarinRhymeShares(m.Phones)
		for j, p := range m.Phones {
			d := base * frontend.PhoneWeight(p.Symbol, p.Role)
			if m.Language == frontend.LanguageChinese {
				if rhymeShares[j] > 0 {
					d = base * mandarinPhoneWeight(m, frontend.Phone{Role: "nucleus"}, 1) * rhymeShares[j]
				}
				if m.Tone == 5 {
					d *= .62
				}
				if i+1 == len(morae) || morae[i+1].Pause {
					d *= 1.12
				}
			}
			result[i][j] = d
		}
	}
	return result
}

func applySpeechScore(cfg Config, morae []frontend.Mora, predictions []prosody.Prediction) []prosody.Prediction {
	durations := speechPhoneDurations(morae, cfg.MoraDurationMS)
	if len(predictions) != len(morae) {
		predictions = make([]prosody.Prediction, len(morae))
		for i := range predictions {
			predictions[i] = prosody.Prediction{DurationFactor: 1, PitchFactor: 1, EnergyFactor: 1}
		}
	}
	for i, m := range morae {
		if m.Pause || len(durations[i]) == 0 {
			continue
		}
		predictions[i].DurationMS = 0
		for _, d := range durations[i] {
			predictions[i].DurationMS += d
		}
	}
	return predictions
}
