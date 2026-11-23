package tts

import (
	"github.com/yh2237/UtauTTS/internal/linguistics"
	"github.com/yh2237/UtauTTS/internal/prosody"
)

func japaneseSpeechPhones(morae []linguistics.Mora) {
	for i := range morae {
		m := &morae[i]
		if m.Pause {
			continue
		}
		m.Language = linguistics.LanguageJapanese
		m.Phones = nil
		if m.Consonant != "" && m.Vowel != "n" && m.Vowel != "cl" {
			m.Phones = append(m.Phones, linguistics.Phone{Symbol: m.Consonant, Role: "onset"})
		}
		m.Phones = append(m.Phones, linguistics.Phone{Symbol: m.Vowel, Role: "nucleus"})
	}
}

func applyJapaneseSpeechRhythm(cfg Config, morae []linguistics.Mora, predictions []prosody.Prediction, features []prosody.FeatureFrame) []prosody.Prediction {
	if cfg.ProsodyPitchOnly {
		return predictions
	}
	return applyJapaneseContextDuration(cfg, morae, features, predictions, finalPhraseIsQuestion(cfg.Text))
}
