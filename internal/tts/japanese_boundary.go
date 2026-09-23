package tts

import (
	"math"

	"utautts/internal/frontend"
	"utautts/internal/prosody"
	"utautts/internal/render"
)

// 句末の境界音調(C2)。最終句の末尾へ控えめな上昇・下降を後処理で加える。
const (
	// boundaryToneWindowMSは境界音調をかける最大区間長。
	boundaryToneWindowMS = 150.0
	// boundaryToneRiseCentsは疑問文の文末上昇。
	boundaryToneRiseCents = 120.0
	// boundaryToneFallCentsは平叙文の文末下降。
	boundaryToneFallCents = -70.0
	// maxBoundaryToneStrengthは境界音調の強度上限。
	maxBoundaryToneStrength = 2.0
)

// applyBoundaryToneは自動F0輪郭の終端付近へ滑らかな境界音調を加える。
// durationMSは区間の終端時刻（曲線先頭からのms）。区間長はmin(150ms, durationMS)。
// 立ち上がりは半余弦で、durationMS以降のフレームは最大値を保持して曲線の段差を避ける。
// strengthは偏差に乗算し、0以下は恒等。出力のフレーム数は変えない。
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

// boundaryToneEnabledはC2が有効かを返す。未指定(nil)は既定ON。
func boundaryToneEnabled(cfg Config) bool {
	return cfg.BoundaryTone == nil || *cfg.BoundaryTone
}

// boundaryToneStrengthは適用強度を返す。0は既定1.0、負値はそのまま返す。
func boundaryToneStrength(cfg Config) float64 {
	if cfg.BoundaryToneStrength == 0 {
		return 1
	}
	return cfg.BoundaryToneStrength
}

// finalPhraseEndMSは最終発話モーラの終了時刻を返す。発話モーラが無ければ0。
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
