package tts

import (
	"fmt"

	"github.com/yh2237/UtauTTS/internal/linguistics"
	"github.com/yh2237/UtauTTS/internal/prosody"
	"github.com/yh2237/UtauTTS/internal/render"
)

// 波形合成での言語固有の韻律・音素長処理をまとめる。G2Pとエイリアス解決は別層が担当する。
type languageProfile interface {
	Language() string
	ApplySpeechProfile(cfg *Config)
	SupportsStretchAdapt() bool
	// 無効時はnilを返す。
	PhoneTiming(cfg Config, morae []linguistics.Mora, singleCV bool) ([][]float64, string)
	Predict(morae []linguistics.Mora) []prosody.Prediction
	AdjustPredictions(cfg Config, model *prosody.Model, morae []linguistics.Mora, predictions []prosody.Prediction, features []prosody.FeatureFrame) []prosody.Prediction
	// enablePitchはピッチ適用を強制するかを表す。
	AutomaticPitchCurve(cfg Config, model *prosody.Model, morae []linguistics.Mora, timings []prosody.MoraTiming, durationMS float64) (*render.PitchCurve, bool)
	ApplyBoundaryTone(cfg Config, curve *render.PitchCurve, durationMS float64, question bool) *render.PitchCurve
}

func languageProfileFor(language string) languageProfile {
	normalized := linguistics.NormalizeLanguage(language)
	if normalized == linguistics.LanguageJapanese {
		return japaneseProfile{}
	}
	return defaultWaveformProfile{language: normalized}
}

type defaultWaveformProfile struct {
	language string
}

func (p defaultWaveformProfile) Language() string { return p.language }

func (defaultWaveformProfile) ApplySpeechProfile(*Config) {}

func (defaultWaveformProfile) SupportsStretchAdapt() bool { return false }

func (defaultWaveformProfile) PhoneTiming(Config, []linguistics.Mora, bool) ([][]float64, string) {
	return nil, ""
}

func (defaultWaveformProfile) Predict(morae []linguistics.Mora) []prosody.Prediction {
	return make([]prosody.Prediction, len(morae))
}

func (defaultWaveformProfile) AdjustPredictions(_ Config, _ *prosody.Model, _ []linguistics.Mora, predictions []prosody.Prediction, _ []prosody.FeatureFrame) []prosody.Prediction {
	return predictions
}

func (defaultWaveformProfile) AutomaticPitchCurve(Config, *prosody.Model, []linguistics.Mora, []prosody.MoraTiming, float64) (*render.PitchCurve, bool) {
	return nil, false
}

func (defaultWaveformProfile) ApplyBoundaryTone(_ Config, curve *render.PitchCurve, _ float64, _ bool) *render.PitchCurve {
	return curve
}

func predictMorae(cfg Config, profile languageProfile, model *prosody.Model, morae []linguistics.Mora, features []prosody.FeatureFrame) ([]prosody.Prediction, error) {
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
