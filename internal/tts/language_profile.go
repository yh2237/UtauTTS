package tts

import (
	"fmt"
	"os"
	"path/filepath"

	"utautts/internal/frontend"
	"utautts/internal/prosody"
	"utautts/internal/render"
)

// languageProfileは言語ごとの振る舞いをまとめる。オーケストレーターは
// profileの選択と共通処理の実行だけを行い、言語分岐を持たない。
// renderer固有の判定は持たず、capability経由の共通処理へ委ねる。
type languageProfile interface {
	// Languageは言語コードを返す。
	Language() string
	// ParsePronunciationはphonemizerごとの発音解析を行う。
	ParsePronunciation(cfg Config, phonemizer string) (string, []frontend.Mora, error)
	// ApplySpeechProfileは言語固有の描画設定をConfigへ反映する。
	ApplySpeechProfile(cfg *Config)
	// ProsodyModelFallbackは指定モデルが言語非対応のときの代替パスを返す。無ければ空。
	ProsodyModelFallback(configuredPath string) string
	// SupportsStretchAdaptは伸縮の音源適応(C3a)の対象かを返す。
	SupportsStretchAdapt() bool
	// PhoneTimingは言語phone weightsとその出所を返す。無効時はnil。
	PhoneTiming(cfg Config, morae []frontend.Mora, singleCV bool) ([][]float64, string)
	// Predictは言語規則による基本予測を返す。無ければnil。
	Predict(morae []frontend.Mora) []prosody.Prediction
	// AdjustPredictionsは予測へ言語固有の後処理を加える。
	AdjustPredictions(cfg Config, model *prosody.Model, morae []frontend.Mora, predictions []prosody.Prediction, features []prosody.FeatureFrame) []prosody.Prediction
	// AutomaticPitchCurveは規則ベースの自動F0曲線を返す。enablePitchは描画を強制するか。
	AutomaticPitchCurve(cfg Config, model *prosody.Model, morae []frontend.Mora, timings []prosody.MoraTiming, durationMS float64) (*render.PitchCurve, bool)
	// ApplyBoundaryToneは自動曲線へ言語固有の境界音調を加える。
	ApplyBoundaryTone(cfg Config, curve *render.PitchCurve, durationMS float64, question bool) *render.PitchCurve
	// ExperimentalPitchAllowedはapplyPitch無効でもpitch実験を適用するか（声調言語）。
	ExperimentalPitchAllowed() bool
}

// languageProfileForは言語コードに対応するprofileを返す。未知は日本語として扱う。
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

// predictMoraeは言語profileに沿って予測を組み立てる。SynthesizeとPredictProsodyで共有する。
func predictMorae(cfg Config, profile languageProfile, model *prosody.Model, morae []frontend.Mora, features []prosody.FeatureFrame) ([]prosody.Prediction, error) {
	predictions := profile.Predict(morae)
	if experimentalSpeechTiming(cfg) {
		predictions = speechRhythmExperiment(morae, predictions, cfg.MoraDurationsMS)
	}
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
	return profile.AdjustPredictions(cfg, model, morae, predictions, features), nil
}

// resolveProsodyModelForProfileは言語profileに応じて代替モデルを探す。
func resolveProsodyModelForProfile(cfg Config, profile languageProfile) (*prosody.Model, error) {
	model, err := resolveProsodyModel(cfg)
	if err != nil || model == nil || model.SupportsLanguage(profile.Language()) {
		return model, err
	}
	if cfg.ProsodyModel != nil || cfg.ProsodyModelPath == "" {
		return nil, nil
	}
	path := profile.ProsodyModelFallback(cfg.ProsodyModelPath)
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

// englishFallbackProsodyModelPathは同じmodelsディレクトリの英語モデルを返す。
func englishFallbackProsodyModelPath(configuredPath string) string {
	return filepath.Join(filepath.Dir(configuredPath), "english-intonation-v1.json")
}
