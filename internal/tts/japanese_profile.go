package tts

import (
	"fmt"

	"utautts/internal/frontend"
	"utautts/internal/prosody"
	"utautts/internal/render"
)

// japaneseProfileは日本語のかな読みと文脈連動・境界音調をまとめる。
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
	if cfg.SpeechTiming {
		japaneseSpeechPhones(morae)
	}
	return reading, morae, err
}

func (japaneseProfile) ApplySpeechProfile(*Config) {}

func (japaneseProfile) ProsodyModelFallback(string) string { return "" }

func (japaneseProfile) SupportsStretchAdapt() bool { return true }

func (japaneseProfile) PhoneTiming(cfg Config, morae []frontend.Mora, singleCV bool) ([][]float64, string) {
	if !cfg.SpeechTiming && singleCV {
		japaneseSpeechPhones(morae)
	}
	if !shouldUseLanguagePhoneTiming(frontend.LanguageJapanese, cfg.SpeechTiming, singleCV) {
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

func (japaneseProfile) ExperimentalPitchAllowed() bool { return false }
