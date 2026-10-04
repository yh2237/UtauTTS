package tts

import (
	"fmt"

	"utautts/internal/frontend"
	"utautts/internal/prosody"
	"utautts/internal/render"
)

type japaneseProfile struct{}

func (japaneseProfile) Language() string { return frontend.LanguageJapanese }

func (japaneseProfile) ParsePronunciation(cfg Config, phonemizer string) (string, []frontend.Mora, error) {
	if phonemizer != frontend.PhonemizerJapanese {
		return "", nil, fmt.Errorf("unsupported phonemizer %q for language %q", phonemizer, frontend.LanguageJapanese)
	}
	reading, err := resolveReading(cfg)
	if err != nil {
		return "", nil, err
	}
	morae, err := frontend.ParseKana(reading)
	return reading, morae, err
}

func (japaneseProfile) ApplySpeechProfile(*Config) {}

func (japaneseProfile) ProsodyModelFallback(string) string { return "" }

func (japaneseProfile) SupportsStretchAdapt() bool { return true }

func (japaneseProfile) PhoneTiming(cfg Config, morae []frontend.Mora, singleCV bool) ([][]float64, string) {
	if singleCV {
		japaneseSpeechPhones(morae)
	}
	if !shouldUseLanguagePhoneTiming(frontend.LanguageJapanese, singleCV) {
		return nil, ""
	}
	return languagePhoneWeights(frontend.LanguageJapanese, morae), "language-phone-v1"
}

func (japaneseProfile) Predict([]frontend.Mora) []prosody.Prediction { return nil }

func (japaneseProfile) AdjustPredictions(cfg Config, model *prosody.Model, morae []frontend.Mora, predictions []prosody.Prediction, features []prosody.FeatureFrame) []prosody.Prediction {
	return applyJapaneseSpeechRhythm(cfg, model, morae, predictions, features)
}

func (japaneseProfile) AutomaticPitchCurve(Config, *prosody.Model, []frontend.Mora, []prosody.MoraTiming, float64) (*render.PitchCurve, bool) {
	return nil, false
}

func (japaneseProfile) ApplyBoundaryTone(cfg Config, curve *render.PitchCurve, durationMS float64, question bool) *render.PitchCurve {
	if !boundaryToneEnabled(cfg) {
		return curve
	}
	return applyBoundaryTone(curve, durationMS, question, boundaryToneStrength(cfg))
}

