package tts

import (
	"fmt"
	"os"
	"path/filepath"

	"utautts/internal/frontend"
	"utautts/internal/prosody"
	"utautts/internal/render"
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
		return englishProfile{}
	case frontend.LanguageChinese:
		return chineseProfile{}
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
		if model.MandarinIntonation == nil {
			predictions = model.PredictWithFeatures(morae, features)
		}
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
	if cfg.ProsodyModel != nil || cfg.ProsodyModelPath == "" {
		return nil, nil
	}
	path := prosodyFallbackModelPath(cfg.ProsodyModelPath, profile.Language())
	if path == "" {
		return nil, nil
	}
	if _, statErr := os.Stat(path); statErr != nil {
		if os.IsNotExist(statErr) {
			return nil, nil
		}
		return nil, statErr
	}
	return loadProsodyModelCached(path)
}

// 言語別の代替抑揚モデル。configuredPathと同じディレクトリから読む。
var prosodyModelFallbacks = map[string]string{
	frontend.LanguageEnglish: "frame-intonation-tcn-en-v1.json",
	frontend.LanguageChinese: "tone-intonation-zh-v1.json",
}

func prosodyFallbackModelPath(configuredPath, language string) string {
	name := prosodyModelFallbacks[frontend.NormalizeLanguage(language)]
	if name == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(configuredPath), name)
}
