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
		m.Phones = nil
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
	return japaneseSpeechRhythmPredictions(morae, predictions)
}

func diffsingerSpeechRhythm(cfg Config, model *prosody.Model, morae []frontend.Mora, predictions []prosody.Prediction) []prosody.Prediction {
	if cfg.ProsodyPitchOnly || (model != nil && (model.MoraDuration != nil || len(model.DurationWeights) > 0)) {
		return predictions
	}
	return japaneseSpeechRhythmPredictions(morae, predictions)
}

func applyJapaneseSpeechRhythm(cfg Config, model *prosody.Model, morae []frontend.Mora, predictions []prosody.Prediction, features []prosody.FeatureFrame) []prosody.Prediction {
	if cfg.ProsodyPitchOnly {
		return predictions
	}
	if cfg.Renderer == "diffsinger" {
		predictions = diffsingerSpeechRhythm(cfg, model, morae, predictions)
	} else {
		predictions = japaneseSpeechRhythm(cfg, model, morae, predictions)
	}
	// 韻律特徴に基づく文脈連動のモーラ長は既定で適用する。
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
