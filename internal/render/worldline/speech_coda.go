package worldline

import (
	"math"

	"utautts/internal/frontend"
	"utautts/internal/plan"
	"utautts/internal/provider"
	"utautts/internal/render/base"
	"utautts/internal/speechwindow"
)

// 固定部の候補が弱ければ、信頼度の高い立ち上がりを保護する。
func speechCodaTransient(p *plan.Plan, u plan.Unit, sourceEnd float64) (float64, float64, bool) {
	if p.Language != frontend.LanguageEnglish || !base.CodaReleaseStop(u) || u.SpeechProfile == nil {
		return 0, 0, false
	}
	profile := u.SpeechProfile
	position, duration, confidence := profile.ReleaseTransientMS, profile.ReleaseTransientDurationMS, profile.ReleaseTransientConfidence
	onsetReliable := profile.TransientConfidence >= stopTransientFloor &&
		profile.TransientMS > 0 && profile.TransientMS < sourceEnd && profile.TransientDurationMS > 0
	if confidence < stopReleaseTransientFloor || position <= 0 ||
		onsetReliable && profile.TransientConfidence >= .65 && profile.TransientConfidence > confidence+.25 {
		position, duration, confidence = profile.TransientMS, profile.TransientDurationMS, profile.TransientConfidence
		if confidence < stopTransientFloor {
			return 0, 0, false
		}
	}
	if math.IsNaN(position) || math.IsInf(position, 0) || position <= 0 || position >= sourceEnd || duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0) {
		return 0, 0, false
	}
	return position, duration, true
}

// 発話時刻を保ち、検出した過渡区間は元の速度で写す。
func protectSpeechTransient(anchors []provider.SpeechAnchor, position, duration float64) ([]provider.SpeechAnchor, bool) {
	if len(anchors) < 2 {
		return anchors, false
	}
	first, last := anchors[0], anchors[len(anchors)-1]
	left := position - speechwindow.TransientLeadMS
	right := position + speechwindow.TransientTailMS(duration, 10)
	if left <= first.SourceMS+.01 || right >= last.SourceMS-.01 || last.TargetMS-first.TargetMS <= right-left+4 {
		return anchors, false
	}
	target := first.TargetMS
	for i := 1; i < len(anchors); i++ {
		a, b := anchors[i-1], anchors[i]
		if left <= b.SourceMS {
			target = a.TargetMS + (left-a.SourceMS)*(b.TargetMS-a.TargetMS)/(b.SourceMS-a.SourceMS)
			break
		}
	}
	target = math.Max(first.TargetMS+1, math.Min(last.TargetMS-(right-left)-5, target))
	result := make([]provider.SpeechAnchor, 0, len(anchors)+2)
	for _, a := range anchors {
		if a.SourceMS < left && a.TargetMS < target {
			result = append(result, a)
		}
	}
	result = append(result, provider.SpeechAnchor{SourceMS: left, TargetMS: target}, provider.SpeechAnchor{SourceMS: right, TargetMS: target + right - left})
	for _, a := range anchors {
		if a.SourceMS > right && a.TargetMS > target+right-left {
			result = append(result, a)
		}
	}
	return result, true
}
