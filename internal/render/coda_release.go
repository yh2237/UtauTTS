package render

import (
	"utautts/internal/frontend"
	"utautts/internal/plan"
)

// 必須の語末子音は音源のエイリアスではなく発音解析結果で判定する。
func worldCodaReleaseEligible(p *plan.Plan, u plan.Unit) bool {
	return (p.Phonemizer == frontend.PhonemizerEnglishDelta || p.Phonemizer == frontend.PhonemizerEnglishVCCV) && u.Role == "ending" && len(u.CodaPhones) > 0
}
