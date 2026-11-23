package tts

import (
	"strings"

	"github.com/yh2237/UtauTTS/internal/linguistics"
	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/prosody"
)

// 読み上げ向けに、文脈による伸縮を控えめにする。
const (
	japaneseContextMinFactor        = 0.8
	japaneseContextMaxFactor        = 1.3
	japaneseAccentPhraseEndFactor   = 1.15
	japanesePhraseFinalFactor       = 1.2
	japaneseQuestionFinalFactor     = 1.25
	japaneseParticleFactor          = 0.85
	japaneseContentWordStartFactor  = 1.05
	japaneseContextBoundaryPresence = 0.5 // 0/1特徴の真偽判定しきい値
)

// 特徴がない、またはモーラ数と合わない場合は全て1倍にする。
func japaneseContextDurationFactors(morae []linguistics.Mora, features []prosody.FeatureFrame, question bool) []float64 {
	factors := make([]float64, len(morae))
	for i := range factors {
		factors[i] = 1
	}
	if len(morae) == 0 || len(features) != len(morae) || !hasJapaneseContextFeatures(features) {
		return factors
	}
	for i, mora := range morae {
		if mora.Pause {
			continue
		}
		feature := features[i]
		factor := 1.0
		particle := isParticleLikeFeature(feature)
		if particle {
			factor *= japaneseParticleFactor
		}
		switch {
		case plan.IsUtteranceFinalMora(morae, i):
			if question {
				factor *= japaneseQuestionFinalFactor
			} else {
				factor *= japanesePhraseFinalFactor
			}
		case isPhraseFinalMora(morae, i):
			factor *= japanesePhraseFinalFactor
		case feature["accent_phrase_end"] >= japaneseContextBoundaryPresence:
			factor *= japaneseAccentPhraseEndFactor
		}
		if !particle && feature["word_start"] >= japaneseContextBoundaryPresence {
			factor *= japaneseContentWordStartFactor
		}
		factors[i] = clampJapaneseContextFactor(factor)
	}
	return factors
}

// 強度が0以下なら補正しない。
func applyJapaneseContextDuration(cfg Config, morae []linguistics.Mora, features []prosody.FeatureFrame, predictions []prosody.Prediction, question bool) []prosody.Prediction {
	if !cfg.ContextDuration {
		return predictions
	}
	strength := cfg.ContextDurationStrength
	if strength <= 0 {
		return predictions
	}
	if len(morae) == 0 {
		return predictions
	}
	if len(predictions) == 0 {
		predictions = make([]prosody.Prediction, len(morae))
		for i := range predictions {
			predictions[i] = prosody.Prediction{DurationFactor: 1, PitchFactor: 1, EnergyFactor: 1}
		}
	}
	factors := japaneseContextDurationFactors(morae, features, question)
	for i := range morae {
		if i >= len(predictions) {
			break
		}
		if factors[i] <= 0 {
			continue
		}
		predictions[i].DurationFactor *= 1 + (factors[i]-1)*strength
	}
	return predictions
}

func hasJapaneseContextFeatures(features []prosody.FeatureFrame) bool {
	for _, feature := range features {
		for key, value := range feature {
			if value < japaneseContextBoundaryPresence {
				continue
			}
			if strings.HasPrefix(key, "accent_") || strings.HasPrefix(key, "word_") ||
				strings.HasPrefix(key, "pos=") || strings.HasPrefix(key, "pos_group1=") {
				return true
			}
		}
	}
	return false
}

func isParticleLikeFeature(feature prosody.FeatureFrame) bool {
	for key, value := range feature {
		if value < japaneseContextBoundaryPresence {
			continue
		}
		if pos, ok := strings.CutPrefix(key, "pos="); ok {
			if pos == "助詞" || pos == "助動詞" {
				return true
			}
		}
		if group, ok := strings.CutPrefix(key, "pos_group1="); ok {
			if strings.Contains(group, "助詞") || strings.Contains(group, "助動詞") {
				return true
			}
		}
	}
	return false
}

func isPhraseFinalMora(morae []linguistics.Mora, index int) bool {
	if index+1 >= len(morae) {
		return true
	}
	return morae[index+1].Pause
}

func clampJapaneseContextFactor(factor float64) float64 {
	if factor < japaneseContextMinFactor {
		return japaneseContextMinFactor
	}
	if factor > japaneseContextMaxFactor {
		return japaneseContextMaxFactor
	}
	return factor
}
