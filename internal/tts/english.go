package tts

import (
	"fmt"

	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/prosody"
	"github.com/yh2237/UtauTTS/internal/render"
)

// ReferenceDirはモデルのフォルダの下で、参照TTSと特徴量のモデルを置くフォルダ。
// いまは英語の参照モデル（KokoroとWavLM）が入っている。
const ReferenceDir = "english-reference"

type englishProfile struct{}

func (englishProfile) Language() string { return frontend.LanguageEnglish }

func (englishProfile) ParsePronunciation(cfg Config, phonemizer string) (string, []frontend.Mora, error) {
	switch phonemizer {
	case frontend.PhonemizerEnglish:
		return frontend.ParseEnglishARPAsing(cfg.Text, cfg.Reading, cfg.Dictionary)
	case frontend.PhonemizerEnglishDelta:
		var presamp frontend.PresampConfig
		if cfg.Voicebank != nil {
			presamp = cfg.Voicebank.Presamp.FrontendConfig()
		}
		return frontend.ParseEnglishDeltaWithConfig(cfg.Text, cfg.Reading, cfg.Dictionary, presamp)
	case frontend.PhonemizerEnglishVCCV:
		return frontend.ParseEnglishVCCV(cfg.Text, cfg.Reading, cfg.Dictionary)
	case frontend.PhonemizerEnglishCV:
		return frontend.ParseEnglishCV(cfg.Text, cfg.Reading, cfg.Dictionary)
	default:
		return "", nil, fmt.Errorf("unsupported phonemizer %q for language %q", phonemizer, frontend.LanguageEnglish)
	}
}

// 英語は参照TTSで合成するので（english_reference.go）、原音接続用の処理は持たない。
func (englishProfile) ApplySpeechProfile(*Config) {}

func (englishProfile) SupportsStretchAdapt() bool { return false }

func (englishProfile) PhoneTiming(Config, []frontend.Mora, bool) ([][]float64, string) {
	return nil, ""
}

func (englishProfile) Predict(morae []frontend.Mora) []prosody.Prediction {
	return make([]prosody.Prediction, len(morae))
}

func (englishProfile) AdjustPredictions(_ Config, _ *prosody.Model, _ []frontend.Mora, predictions []prosody.Prediction, _ []prosody.FeatureFrame) []prosody.Prediction {
	return predictions
}

func (englishProfile) AutomaticPitchCurve(Config, *prosody.Model, []frontend.Mora, []prosody.MoraTiming, float64) (*render.PitchCurve, bool) {
	return nil, false
}

func (englishProfile) ApplyBoundaryTone(_ Config, curve *render.PitchCurve, _ float64, _ bool) *render.PitchCurve {
	return curve
}
