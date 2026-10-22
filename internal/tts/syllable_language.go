package tts

import (
	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/prosody"
	"github.com/yh2237/UtauTTS/internal/render"
)

// syllableProfileは、音節を1モーラにして読む言語（スペイン語、ポルトガル語）。
// 参照照合だけで合成し、原音接続用の処理は持たない。
type syllableProfile struct{ language string }

func (p syllableProfile) Language() string { return p.language }

func (p syllableProfile) ParsePronunciation(cfg Config, phonemizer string) (string, []frontend.Mora, error) {
	return frontend.ParseSyllableLanguage(p.language, phonemizer, cfg.Text, cfg.Reading, cfg.Dictionary)
}

func (syllableProfile) ApplySpeechProfile(*Config) {}

func (syllableProfile) SupportsStretchAdapt() bool { return false }

func (syllableProfile) PhoneTiming(Config, []frontend.Mora, bool) ([][]float64, string) {
	return nil, ""
}

func (syllableProfile) Predict(morae []frontend.Mora) []prosody.Prediction {
	return make([]prosody.Prediction, len(morae))
}

func (syllableProfile) AdjustPredictions(_ Config, _ *prosody.Model, _ []frontend.Mora, predictions []prosody.Prediction, _ []prosody.FeatureFrame) []prosody.Prediction {
	return predictions
}

func (syllableProfile) AutomaticPitchCurve(Config, *prosody.Model, []frontend.Mora, []prosody.MoraTiming, float64) (*render.PitchCurve, bool) {
	return nil, false
}

func (syllableProfile) ApplyBoundaryTone(_ Config, curve *render.PitchCurve, _ float64, _ bool) *render.PitchCurve {
	return curve
}
