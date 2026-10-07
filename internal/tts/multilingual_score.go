package tts

import (
	"math"
	"utautts/internal/frontend"
	"utautts/internal/plan"
	"utautts/internal/prosody"
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
		final := true
		for j := i + 1; j < len(morae) && !morae[j].Pause; j++ {
			if morae[j].WordIndex != m.WordIndex {
				final = false
				break
			}
		}
		for j, p := range m.Phones {
			d := base * frontend.PhoneWeight(p.Symbol, p.Role)
			if m.Language == frontend.LanguageEnglish {
				if p.Role == "nucleus" {
					if m.Stress == 1 {
						d *= 1.2
					} else if m.Stress == 2 {
						d *= 1.1
					} else if m.StressKnown {
						d *= .85
					}
					if final {
						d *= 1.12
					}
				} else if p.Role == "coda" {
					d = base * englishPhoneWeight(m, p, frontend.PhoneWeight(p.Symbol, p.Role))
					d = math.Max(d, base*.5)
				}
			} else if m.Language == frontend.LanguageChinese {
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
