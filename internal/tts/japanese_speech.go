package tts

import (
	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/prosody"
	"strings"
)

func japaneseSpeechPhones(morae []frontend.Mora) {
	for i := range morae {
		m := &morae[i]
		if m.Pause {
			continue
		}
		m.Language = frontend.LanguageJapanese
		m.Phones = nil
		if m.Consonant != "" && m.Vowel != "n" && m.Vowel != "cl" {
			m.Phones = append(m.Phones, frontend.Phone{Symbol: m.Consonant, Role: "onset"})
		}
		m.Phones = append(m.Phones, frontend.Phone{Symbol: m.Vowel, Role: "nucleus"})
	}
}

func internalTimingSpeechRhythm(cfg Config, morae []frontend.Mora, predictions []prosody.Prediction) []prosody.Prediction {
	if cfg.ProsodyPitchOnly {
		return predictions
	}
	return japaneseSpeechRhythmPredictions(morae, predictions)
}

func applyJapaneseSpeechRhythm(cfg Config, morae []frontend.Mora, predictions []prosody.Prediction, features []prosody.FeatureFrame) []prosody.Prediction {
	if cfg.ProsodyPitchOnly {
		return predictions
	}
	if rendererInternalTiming(cfg.RendererCapabilities) {
		predictions = internalTimingSpeechRhythm(cfg, morae, predictions)
	}
	return applyJapaneseContextDuration(cfg, morae, features, predictions, finalPhraseIsQuestion(cfg.Text))
}

func japaneseSpeechRhythmPredictions(morae []frontend.Mora, predictions []prosody.Prediction) []prosody.Prediction {
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
		// 無声化しやすい文脈だけ短くし、録音母音は無音にしない。
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
