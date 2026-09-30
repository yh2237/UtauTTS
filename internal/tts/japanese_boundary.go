package tts

import (
	"math"

	"utautts/internal/frontend"
	"utautts/internal/prosody"
	"utautts/internal/render"
)

const (
	boundaryToneWindowMS    = 150.0
	boundaryToneRiseCents   = 120.0
	boundaryToneFallCents   = -70.0
	maxBoundaryToneStrength = 2.0
)

// durationMSは曲線先頭からの終端時刻。終端後も補正値を保ち、段差を防ぐ。
func applyBoundaryTone(curve *render.PitchCurve, durationMS float64, question bool, strength float64) *render.PitchCurve {
	if curve == nil || len(curve.Cents) == 0 || curve.FrameMS <= 0 || math.IsNaN(curve.FrameMS) {
		return curve
	}
	if strength <= 0 {
		return curve
	}
	if strength > maxBoundaryToneStrength {
		strength = maxBoundaryToneStrength
	}
	deviation := boundaryToneFallCents
	if question {
		deviation = boundaryToneRiseCents
	}
	deviation *= strength

	window := boundaryToneWindowMS
	if durationMS < window {
		window = durationMS
	}
	if window <= 0 {
		return curve
	}
	startMS := durationMS - window

	result := &render.PitchCurve{FrameMS: curve.FrameMS, Cents: append([]float64(nil), curve.Cents...)}
	for index := range result.Cents {
		timeMS := float64(index) * curve.FrameMS
		if timeMS < startMS {
			continue
		}
		// 区間内は半余弦で0から最大へ立ち上げ、区間より後は最大値を保持する。
		weight := 1.0
		if timeMS < durationMS {
			progress := (timeMS - startMS) / window
			if progress < 0 {
				progress = 0
			} else if progress > 1 {
				progress = 1
			}
			weight = 0.5 * (1 - math.Cos(math.Pi*progress))
		}
		result.Cents[index] += deviation * weight
	}
	return result
}

func boundaryToneEnabled(cfg Config) bool {
	return cfg.BoundaryTone == nil || *cfg.BoundaryTone
}

func boundaryToneStrength(cfg Config) float64 {
	if cfg.BoundaryToneStrength == 0 {
		return 1
	}
	return cfg.BoundaryToneStrength
}

func finalPhraseEndMS(morae []frontend.Mora, timings []prosody.MoraTiming) float64 {
	for index := len(morae) - 1; index >= 0; index-- {
		if morae[index].Pause {
			continue
		}
		if index < len(timings) {
			return timings[index].StartMS + timings[index].DurationMS
		}
		return 0
	}
	return 0
}
