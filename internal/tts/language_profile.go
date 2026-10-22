package tts

import (
	"fmt"

	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/prosody"
	"github.com/yh2237/UtauTTS/internal/render"
)

// 言語固有処理をまとめる。レンダラーの判定は共通処理のcapabilityへ委ねる。
type languageProfile interface {
	Language() string
	ParsePronunciation(cfg Config, phonemizer string) (string, []frontend.Mora, error)
	ApplySpeechProfile(cfg *Config)
	SupportsStretchAdapt() bool
	// 無効時はnilを返す。
	PhoneTiming(cfg Config, morae []frontend.Mora, singleCV bool) ([][]float64, string)
	Predict(morae []frontend.Mora) []prosody.Prediction
	AdjustPredictions(cfg Config, model *prosody.Model, morae []frontend.Mora, predictions []prosody.Prediction, features []prosody.FeatureFrame) []prosody.Prediction
	// enablePitchはピッチ適用を強制するかを表す。
	AutomaticPitchCurve(cfg Config, model *prosody.Model, morae []frontend.Mora, timings []prosody.MoraTiming, durationMS float64) (*render.PitchCurve, bool)
	ApplyBoundaryTone(cfg Config, curve *render.PitchCurve, durationMS float64, question bool) *render.PitchCurve
}

// 未知の言語は日本語として扱う。
func languageProfileFor(language string) languageProfile {
	switch language {
	case frontend.LanguageEnglish:
		return referenceProfile{language: language, parse: parseEnglishPronunciation}
	case frontend.LanguageChinese:
		return referenceProfile{language: language, parse: parseChinesePronunciation}
	case frontend.LanguageSpanish, frontend.LanguagePortuguese:
		return referenceProfile{language: language, parse: parseSyllableLanguage(language)}
	default:
		return japaneseProfile{}
	}
}

func predictMorae(cfg Config, profile languageProfile, model *prosody.Model, morae []frontend.Mora, features []prosody.FeatureFrame) ([]prosody.Prediction, error) {
	predictions := profile.Predict(morae)
	if model != nil {
		if model.RequiresExternalFeatures() && len(features) != len(morae) {
			return nil, fmt.Errorf("prosody model %d/%s requires %d mora-level accent feature frames, got %d", model.Version, model.Mode, len(morae), len(features))
		}
		predictions = model.PredictWithFeatures(morae, features)
		if cfg.ProsodyPitchOnly {
			for i := range predictions {
				predictions[i].DurationMS = 0
				predictions[i].DurationFactor = 1
				predictions[i].EnergyFactor = 1
			}
		}
	}
	predictions = profile.AdjustPredictions(cfg, model, morae, predictions, features)
	return predictions, nil
}

func resolveProsodyModelForProfile(cfg Config, profile languageProfile) (*prosody.Model, error) {
	model, err := resolveProsodyModel(cfg)
	if err != nil || model == nil || model.SupportsLanguage(profile.Language()) {
		return model, err
	}
	return nil, nil
}
