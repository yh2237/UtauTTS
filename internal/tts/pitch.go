package tts

import (
	"math"

	"utautts/internal/prosody"
	"utautts/internal/render"
)

// 自動輪郭を鈍らせないよう、手動補正だけに制限をかける。
// 10msあたり50セントを許容し、モーラ間の手動下降が逆転するのを防ぐ。
func constrainManualPitchContour(manual *prosody.PitchContour) *prosody.PitchContour {
	if manual == nil || manual.FrameMS <= 0 || len(manual.Cents) == 0 {
		return manual
	}
	constrained := render.ConstrainPitchCurve(&render.PitchCurve{FrameMS: manual.FrameMS, Cents: manual.Cents}, 20, 50)
	return &prosody.PitchContour{FrameMS: constrained.FrameMS, Cents: constrained.Cents}
}

func mergeManualPitchCurve(base *render.PitchCurve, manual *prosody.PitchContour, mode string) *render.PitchCurve {
	if manual == nil || manual.FrameMS <= 0 || len(manual.Cents) == 0 {
		return base
	}
	result := &render.PitchCurve{FrameMS: manual.FrameMS, Cents: make([]float64, len(manual.Cents))}
	for index := range result.Cents {
		manualCents := manual.Cents[index]
		if mode == "replace" {
			result.Cents[index] = manualCents
			continue
		}
		baseCents := 0.0
		if base != nil && len(base.Cents) > 0 {
			baseCents = pitchCurveCentsAt(base, float64(index)*manual.FrameMS)
		}
		result.Cents[index] = baseCents + manualCents
	}
	return result
}

func pitchCurveCentsAt(curve *render.PitchCurve, timeMS float64) float64 {
	return curve.CentsAt(timeMS)
}

// 音源ピッチの安定化とは別の上限。2を超える分は大きな輪郭変化だけを広げる。
const MaxIntonationStrength = 8.0

const (
	// intonationExpandBaseは一律に倍率を掛ける強さの上限。これを超える分は大きな動きだけを広げる。
	intonationExpandBase = 2.0
	// intonationExpandCentsは、強さ2の曲線で広げ始める動きの大きさの目安（セント）。
	intonationExpandCents = 100.0
)

// 手動補正は増幅しない。強さ2より上では、平らな部分を保ち大きな動きだけ広げる。
func scaleAutomaticPitchCurve(curve *render.PitchCurve, strength float64) *render.PitchCurve {
	if curve == nil || len(curve.Cents) == 0 {
		return curve
	}
	if strength <= 0 {
		return nil
	}
	if strength == 1 {
		return curve
	}
	result := &render.PitchCurve{FrameMS: curve.FrameMS, Cents: make([]float64, len(curve.Cents))}
	for index, cents := range curve.Cents {
		if strength <= intonationExpandBase {
			result.Cents[index] = cents * strength
			continue
		}
		base := cents * intonationExpandBase
		ratio := base / intonationExpandCents
		result.Cents[index] = base * (1 + (strength/intonationExpandBase-1)*(1-math.Exp(-ratio*ratio)))
	}
	return result
}
