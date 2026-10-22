package tts

import (
	"fmt"

	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/prosody"
	"github.com/yh2237/UtauTTS/internal/render"
)

// スペイン語は参照照合だけで合成する。原音接続用の処理は持たない。
type spanishProfile struct{}

func (spanishProfile) Language() string { return frontend.LanguageSpanish }

func (spanishProfile) ParsePronunciation(cfg Config, phonemizer string) (string, []frontend.Mora, error) {
	if phonemizer != frontend.PhonemizerSpanish {
		return "", nil, fmt.Errorf("unsupported phonemizer %q for language %q", phonemizer, frontend.LanguageSpanish)
	}
	return frontend.ParseSpanish(cfg.Text, cfg.Reading, cfg.Dictionary)
}

func (spanishProfile) ApplySpeechProfile(*Config) {}

func (spanishProfile) SupportsStretchAdapt() bool { return false }

func (spanishProfile) PhoneTiming(Config, []frontend.Mora, bool) ([][]float64, string) {
	return nil, ""
}

func (spanishProfile) Predict(morae []frontend.Mora) []prosody.Prediction {
	return make([]prosody.Prediction, len(morae))
}

func (spanishProfile) AdjustPredictions(_ Config, _ *prosody.Model, _ []frontend.Mora, predictions []prosody.Prediction, _ []prosody.FeatureFrame) []prosody.Prediction {
	return predictions
}

func (spanishProfile) AutomaticPitchCurve(Config, *prosody.Model, []frontend.Mora, []prosody.MoraTiming, float64) (*render.PitchCurve, bool) {
	return nil, false
}

func (spanishProfile) ApplyBoundaryTone(_ Config, curve *render.PitchCurve, _ float64, _ bool) *render.PitchCurve {
	return curve
}
