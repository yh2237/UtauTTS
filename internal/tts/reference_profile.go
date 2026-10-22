package tts

import (
	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/prosody"
	"github.com/yh2237/UtauTTS/internal/render"
)

// referenceProfileは、参照照合で読む言語（英語、スペイン語、ポルトガル語）。
// 長さと抑揚は参照TTSの読みから決めるので、原音接続用の処理は持たない。
type referenceProfile struct {
	language string
	parse    func(cfg Config, phonemizer string) (string, []frontend.Mora, error)
}

func (p referenceProfile) Language() string { return p.language }

func (p referenceProfile) ParsePronunciation(cfg Config, phonemizer string) (string, []frontend.Mora, error) {
	return p.parse(cfg, phonemizer)
}

func (referenceProfile) ApplySpeechProfile(*Config) {}

func (referenceProfile) SupportsStretchAdapt() bool { return false }

func (referenceProfile) PhoneTiming(Config, []frontend.Mora, bool) ([][]float64, string) {
	return nil, ""
}

func (referenceProfile) Predict(morae []frontend.Mora) []prosody.Prediction {
	return make([]prosody.Prediction, len(morae))
}

func (referenceProfile) AdjustPredictions(_ Config, _ *prosody.Model, _ []frontend.Mora, predictions []prosody.Prediction, _ []prosody.FeatureFrame) []prosody.Prediction {
	return predictions
}

func (referenceProfile) AutomaticPitchCurve(Config, *prosody.Model, []frontend.Mora, []prosody.MoraTiming, float64) (*render.PitchCurve, bool) {
	return nil, false
}

func (referenceProfile) ApplyBoundaryTone(_ Config, curve *render.PitchCurve, _ float64, _ bool) *render.PitchCurve {
	return curve
}

// parseSyllableLanguageは、音節を1モーラにして読む言語（スペイン語、ポルトガル語）の読みを返す。
func parseSyllableLanguage(language string) func(Config, string) (string, []frontend.Mora, error) {
	return func(cfg Config, phonemizer string) (string, []frontend.Mora, error) {
		return frontend.ParseSyllableLanguage(language, phonemizer, cfg.Text, cfg.Reading, cfg.Dictionary)
	}
}
