package tts

import (
	"strings"
	"utautts/internal/frontend"
	"utautts/internal/prosody"
)

func japaneseSpeechPhones(morae []frontend.Mora) {
	for i := range morae {
		m := &morae[i]
		if m.Pause {
			continue
		}
		m.Language = frontend.LanguageJapanese
		if m.Consonant != "" && m.Vowel != "n" && m.Vowel != "cl" {
			m.Phones = append(m.Phones, frontend.Phone{Symbol: m.Consonant, Role: "onset"})
		}
		m.Phones = append(m.Phones, frontend.Phone{Symbol: m.Vowel, Role: "nucleus"})
	}
}

func japaneseSpeechRhythm(cfg Config, model *prosody.Model, morae []frontend.Mora, predictions []prosody.Prediction) []prosody.Prediction {
	if !cfg.SpeechTiming || cfg.ProsodyPitchOnly || (model != nil && (model.MoraDuration != nil || len(model.DurationWeights) > 0)) {
		return predictions
	}
	if len(predictions) == 0 {
		predictions = make([]prosody.Prediction, len(morae))
		for i := range predictions {
			predictions[i] = prosody.Prediction{DurationFactor: 1, PitchFactor: 1, EnergyFactor: 1}
		}
	}
	for i, m := range morae {
		if m.Pause {
			continue
		}
		factor := 1.0
		if m.Consonant != "" {
			factor = (1 + frontend.PhoneWeight(m.Consonant, "onset")) / 1.45
		}
		// Shorten likely devoicing contexts without forcing recorded vowels silent.
		unvoiced := func(c string) bool { return c != "" && strings.Contains(" k ky s sh t ts ch h hy f p py ", " "+c+" ") }
		if (m.Vowel == "i" || m.Vowel == "u") && unvoiced(m.Consonant) && i+1 < len(morae) && !morae[i+1].Pause && unvoiced(morae[i+1].Consonant) {
			factor *= 0.85
		}
		if i+1 == len(morae) || morae[i+1].Pause {
			factor *= 1.15
		}
		predictions[i].DurationFactor = factor
	}
	return predictions
}
