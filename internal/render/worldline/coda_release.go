package worldline

import (
	"math"

	"utautts/internal/frontend"
	"utautts/internal/plan"
	"utautts/internal/render/base"
)

// 必須の語末子音は音源のエイリアスではなく発音解析結果で判定する。
func worldCodaReleaseEligible(p *plan.Plan, u plan.Unit) bool {
	if len(u.CodaPhones) == 0 {
		return false
	}
	switch p.Phonemizer {
	case frontend.PhonemizerEnglishDelta, frontend.PhonemizerEnglishVCCV:
		return u.Role == "ending"
	case frontend.PhonemizerEnglish, frontend.PhonemizerEnglishCV, frontend.PhonemizerChinese:
		return u.Role == "mora"
	}
	return false
}

func codaReleaseEnvelope(u plan.Unit, points []base.WorldlineEnvelopePoint, fadeOut float64) ([]base.WorldlineEnvelopePoint, float64) {
	if len(u.CodaPhones) == 0 || u.DurationMS <= 0 {
		return points, fadeOut
	}
	fadeOut = math.Min(fadeOut, math.Max(2, math.Min(5, u.DurationMS*.25)))
	if len(points) != 5 {
		return points, fadeOut
	}
	result := append([]base.WorldlineEnvelopePoint(nil), points...)
	result[3].XMS = math.Max(result[2].XMS, result[4].XMS-fadeOut)
	return result, fadeOut
}

const (
	codaReleaseMinMS       = 10.0
	codaReleaseMaxMS       = 26.0
	codaReleaseDefaultMS   = 16.0
	codaClosureMinMS       = 12.0
	codaSplitMinDurationMS = 30.0
)

// 過剰な補強を避けるため、日本語破裂音の信頼度下限は高めにする。
const (
	stopTransientFloor         = .5
	stopTransientJapaneseFloor = .65
	stopTransientVCVFloor      = .7
)

// 総長を保ち、測定した解放長と閉鎖の最低長を確保する。
func codaClosureReleaseSplit(u plan.Unit) (float64, float64, bool) {
	if !base.CodaReleaseStop(u) || u.DurationMS < codaSplitMinDurationMS {
		return 0, 0, false
	}
	release := codaReleaseDefaultMS
	if profile := u.SpeechProfile; profile != nil && profile.ReleaseTransientDurationMS > 0 {
		release = profile.ReleaseTransientDurationMS
	}
	release = math.Max(codaReleaseMinMS, math.Min(codaReleaseMaxMS, release))
	release = math.Min(release, u.DurationMS-codaClosureMinMS)
	if release < codaReleaseMinMS {
		return 0, 0, false
	}
	closure := math.Max(codaClosureMinMS, u.DurationMS-release)
	return closure, release, true
}

func worldCodaReleaseSplit(p *plan.Plan, u plan.Unit, options base.WorldlineProviderOptions) (float64, float64, bool) {
	if !worldCodaReleaseEligible(p, u) || !worldlineStopProtection(p, u, options) {
		return 0, 0, false
	}
	return codaClosureReleaseSplit(u)
}

// 解放過渡の測定は信頼度0.25未満で時間を返さないため、保護判定の下限をそれに合わせる。
const stopReleaseTransientFloor = .25
