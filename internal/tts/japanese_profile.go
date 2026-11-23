package tts

import (
	"github.com/yh2237/UtauTTS/internal/linguistics"
	"github.com/yh2237/UtauTTS/internal/prosody"
	"github.com/yh2237/UtauTTS/internal/render"
)

type japaneseProfile struct{}

func (japaneseProfile) Language() string { return linguistics.LanguageJapanese }

func (japaneseProfile) ApplySpeechProfile(*Config) {}

func (japaneseProfile) SupportsStretchAdapt() bool { return true }

func (japaneseProfile) PhoneTiming(cfg Config, morae []linguistics.Mora, singleCV bool) ([][]float64, string) {
	if singleCV {
		japaneseSpeechPhones(morae)
	}
	if !shouldUseLanguagePhoneTiming(linguistics.LanguageJapanese, singleCV) {
		return nil, ""
	}
	return languagePhoneWeights(linguistics.LanguageJapanese, morae), "language-phone-v1"
}

func (japaneseProfile) Predict([]linguistics.Mora) []prosody.Prediction { return nil }

func (japaneseProfile) AdjustPredictions(cfg Config, model *prosody.Model, morae []linguistics.Mora, predictions []prosody.Prediction, features []prosody.FeatureFrame) []prosody.Prediction {
	return applyJapaneseSpeechRhythm(cfg, morae, predictions, features)
}

func (japaneseProfile) AutomaticPitchCurve(Config, *prosody.Model, []linguistics.Mora, []prosody.MoraTiming, float64) (*render.PitchCurve, bool) {
	return nil, false
}

func (japaneseProfile) ApplyBoundaryTone(cfg Config, curve *render.PitchCurve, durationMS float64, question bool) *render.PitchCurve {
	if !boundaryToneEnabled(cfg) {
		return curve
	}
	return applyBoundaryTone(curve, durationMS, question, boundaryToneStrength(cfg))
}
