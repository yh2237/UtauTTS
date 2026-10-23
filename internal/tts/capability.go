package tts

import (
	"math"

	"github.com/yh2237/UtauTTS/internal/prosody"
	"github.com/yh2237/UtauTTS/internal/render"
)

func applyPitchEnabled(cfg Config) bool {
	return cfg.ApplyPitch || cfg.ProsodyPitchOnly
}

func shouldPredictFrameContour(cfg Config, model *prosody.Model) bool {
	return applyPitchEnabled(cfg) && model != nil && model.HasFrameContour()
}

func effectiveIntonationStrength(cfg Config) float64 {
	if !applyPitchEnabled(cfg) {
		return 0
	}
	// 自動輪郭が無いときは、強さが音源ピッチ安定化の指数になるため、レンダラーの上限で止める。
	return math.Min(cfg.IntonationStrength, render.MaxIntonationStrength)
}

// 自動曲線使用時も音源由来の補正を弱く残す。
const automaticSourceIntonationBlend = 0.25

func rendererIntonationStrength(cfg Config, automatic *render.PitchCurve) float64 {
	if automatic != nil {
		if !applyPitchEnabled(cfg) {
			return 0
		}
		return automaticSourceIntonationBlend
	}
	return effectiveIntonationStrength(cfg)
}
