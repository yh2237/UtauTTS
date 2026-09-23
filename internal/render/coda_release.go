package render

import (
	"math"
	"strings"

	"utautts/internal/frontend"
	"utautts/internal/plan"
)

// 必須の語末子音は音源のエイリアスではなく発音解析結果で判定する。
func worldCodaReleaseEligible(p *plan.Plan, u plan.Unit) bool {
	return (p.Phonemizer == frontend.PhonemizerEnglishDelta || p.Phonemizer == frontend.PhonemizerEnglishVCCV) && u.Role == "ending" && len(u.CodaPhones) > 0
}

func codaReleaseEnvelope(u plan.Unit, points []worldlineEnvelopePoint, fadeOut float64) ([]worldlineEnvelopePoint, float64) {
	if len(u.CodaPhones) == 0 || u.DurationMS <= 0 {
		return points, fadeOut
	}
	fadeOut = math.Min(fadeOut, math.Max(2, math.Min(5, u.DurationMS*.25)))
	if len(points) != 5 {
		return points, fadeOut
	}
	result := append([]worldlineEnvelopePoint(nil), points...)
	result[3].XMS = math.Max(result[2].XMS, result[4].XMS-fadeOut)
	return result, fadeOut
}

func codaReleaseStop(u plan.Unit) bool {
	for _, phone := range u.CodaPhones {
		if strings.Contains(" p b t d k g ch jh ", " "+strings.ToLower(phone)+" ") {
			return true
		}
	}
	return false
}

// E2a: 英語停止codaの閉鎖と解放の分離に使う有界な定数。
const (
	codaReleaseMinMS       = 10.0
	codaReleaseMaxMS       = 26.0
	codaReleaseDefaultMS   = 16.0
	codaClosureMinMS       = 12.0
	codaSplitMinDurationMS = 30.0
)

// E2b: 日本語破裂音を保護するときの信頼度下限。既存のonset下限より高くして過剰適用を防ぐ。
const (
	stopTransientFloor         = .5
	stopTransientJapaneseFloor = .65
	stopTransientVCVFloor      = .7
)

// codaClosureReleaseSplitは英語停止codaの閉鎖と解放の長さを返す。総長は変えない。
// 解放は測定できた過渡長を優先し、閉鎖には最低長を残す。
func codaClosureReleaseSplit(u plan.Unit) (float64, float64, bool) {
	if !codaReleaseStop(u) || u.DurationMS < codaSplitMinDurationMS {
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

// worldCodaReleaseSplitはE2aが有効で、かつ解放過渡を保護できる停止codaだけ分離を返す。
func worldCodaReleaseSplit(p *plan.Plan, u plan.Unit) (float64, float64, bool) {
	if !e2aEnabled || !worldCodaReleaseEligible(p, u) || !worldlineStopProtection(p, u) {
		return 0, 0, false
	}
	return codaClosureReleaseSplit(u)
}

// 解放過渡の測定は信頼度0.25未満で時間を返さないため、保護判定の下限をそれに合わせる。
const stopReleaseTransientFloor = .25
