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
