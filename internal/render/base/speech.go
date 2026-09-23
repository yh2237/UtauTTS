package base

import (
	"math"
	"strings"

	"utautts/internal/plan"
)

// SpeechRetimeは母音開始をtargetOnsetに合わせ、onsetと安定母音を別アンカーにする。破裂音の短い解放区間は伸縮せずコピーする。
func SpeechRetime(source []float64, targetFrames, sourceOnset, sourceFixed, targetOnset, targetFixed, rate int, stop bool) ([]float64, int, bool) {
	minimum := msToFrames(4, rate)
	if rate <= 0 || minimum < 2 || sourceOnset < minimum || targetOnset < minimum ||
		sourceFixed-sourceOnset < minimum || targetFixed-targetOnset < minimum ||
		len(source)-sourceFixed < msToFrames(20, rate) || targetFrames-targetFixed < msToFrames(20, rate) {
		return nil, targetFixed, false
	}
	// 時間変化は安定母音側で吸収し、子音から安定母音への遷移変化は最大25%までにする。
	ratio := float64(targetFrames-targetOnset) / float64(len(source)-sourceOnset)
	transition := int(math.Round(float64(targetFixed-targetOnset) * math.Max(0.75, math.Min(1.25, math.Sqrt(ratio)))))
	targetFixed = min(targetFrames-msToFrames(20, rate), targetOnset+max(minimum, transition))
	bridge := min(msToFrames(3, rate), minimum)
	prefix := wsola(source[:sourceOnset+bridge], targetOnset+bridge, rate)
	if sourceOnset == targetOnset {
		copy(prefix, source[:sourceOnset+bridge])
	} else if stop {
		protected := min(msToFrames(8, rate), sourceOnset-minimum, targetOnset-minimum)
		if protected > bridge {
			// 母音開始直前の波形を保つ。
			for i := 0; i < protected+bridge; i++ {
				alpha := 1.0
				if i < bridge {
					alpha = float64(i) / float64(bridge)
				}
				dst := targetOnset - protected + i
				prefix[dst] = prefix[dst]*(1-alpha) + source[sourceOnset-protected+i]*alpha
			}
		}
	}
	// 遷移と母音は標準のoverlap対応ストレッチャを再利用する。
	tail, err := RetimeWithCompressedPrefixUsing(source[sourceOnset-bridge:], targetFrames-targetOnset+bridge,
		sourceFixed-sourceOnset+bridge, targetFixed-targetOnset+bridge, rate, WSOLAStretch)
	if err != nil {
		return nil, targetFixed, false
	}
	result := make([]float64, targetFrames)
	copy(result, prefix[:targetOnset-bridge])
	for i := 0; i < 2*bridge; i++ {
		alpha := 0.5 - 0.5*math.Cos(math.Pi*float64(i)/float64(2*bridge-1))
		result[targetOnset-bridge+i] = prefix[targetOnset-bridge+i]*(1-alpha) + tail[i]*alpha
	}
	copy(result[targetOnset+bridge:], tail[2*bridge:])
	return result, targetFixed, true
}

// SpeechStopはユニットのonset/語末が破裂音かを返す。
func SpeechStop(p *plan.Plan, unit plan.Unit) bool {
	// 語末の破裂音は親モーラのonsetとは独立に保護対象にする。
	if CodaReleaseStop(unit) {
		return true
	}
	if unit.Position < 0 || unit.Position >= len(p.Morae) {
		return false
	}
	if isStopPhone(p.Morae[unit.Position].Consonant) {
		return true
	}
	for _, phone := range p.Morae[unit.Position].Phones {
		if phone.Role == "onset" && isStopPhone(phone.Symbol) {
			return true
		}
	}
	return false
}

func isStopPhone(phone string) bool {
	return strings.Contains(" p py b by t d k ky g gy ", " "+strings.ToLower(strings.TrimSpace(phone))+" ")
}

// SpeechPitchAnchorは時間変化するピッチでアンカー位置を写す。ResampleForPitchCurveと同じ積分を使う。
func SpeechPitchAnchor(sourceFrames, anchor int, base float64, curve *PitchCurve, startMS, spanMS float64) int {
	if anchor < 0 || anchor > sourceFrames {
		return -1
	}
	if sourceFrames < 16 || base <= 0 {
		return anchor
	}
	if curve == nil || curve.FrameMS <= 0 || len(curve.Cents) == 0 {
		return int(math.Round(float64(anchor) / clampPitchFactor(base)))
	}
	position := 0.0
	for i := 0; i < anchor; i++ {
		t := startMS + math.Max(1e-3, spanMS)*float64(i)/float64(sourceFrames-1)
		position += 1 / clampPitchFactor(base*PitchCurveFactorAt(curve, t))
	}
	return int(math.Round(position))
}

// SpeechVowelJoinは自動補正が同母音の連続だけに限られるかを返す。
func SpeechVowelJoin(p *plan.Plan, previous, current RenderedUnit) bool {
	if previous.Index+1 != current.Index || previous.Unit.Role != "mora" || current.Unit.Role != "mora" || previous.Unit.Position+1 != current.Unit.Position {
		return false
	}
	if previous.Unit.Position < 0 || current.Unit.Position >= len(p.Morae) {
		return false
	}
	a, b := p.Morae[previous.Unit.Position], p.Morae[current.Unit.Position]
	if a.Pause || b.Pause || a.Vowel == "" || a.Vowel != b.Vowel || a.Vowel == "cl" || a.Vowel == "n" || b.Consonant != "" {
		return false
	}
	for _, phone := range a.Phones {
		if phone.Role == "coda" {
			return false
		}
	}
	for _, phone := range b.Phones {
		if phone.Role == "onset" {
			return false
		}
	}
	for _, unit := range []plan.Unit{previous.Unit, current.Unit} {
		if unit.SpeechProfile == nil || !unit.SpeechProfile.Applied {
			return false
		}
	}
	return true
}

// CodaReleaseStopは語末子音に破裂音を含むかを返す。
func CodaReleaseStop(u plan.Unit) bool {
	for _, phone := range u.CodaPhones {
		if strings.Contains(" p b t d k g ch jh ", " "+strings.ToLower(phone)+" ") {
			return true
		}
	}
	return false
}
