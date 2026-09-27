package worldline

import (
	"math"
	"utautts/internal/plan"
	"utautts/internal/provider"
)

// 原音境界は規則による推定値で、音響的な整列結果ではない。
func mandarinRhymeAnchors(p *plan.Plan, u plan.Unit, sourceEnd, targetEnd float64) ([]provider.SpeechAnchor, bool) {
	pre := math.Max(0, math.Min(sourceEnd-1, u.PreutteranceMS))
	nasalStart := sourceEnd
	nasalTarget := targetEnd
	compound := false
	hasMedial := false
	codaDuration := 0.0
	for _, t := range p.PhoneTimings {
		if t.Position == u.Position {
			if t.Role == "coda" {
				nasalStart = sourceEnd - math.Min(90, sourceEnd*.25)
				codaDuration += t.DurationMS
			}
			if t.Role == "medial" || t.Role == "offglide" {
				compound = true
			}
			if t.Role == "medial" {
				hasMedial = true
			}
		}
	}
	nasalTarget -= codaDuration
	if nasalStart <= pre+4 {
		return nil, false
	}
	var result []provider.SpeechAnchor
	for _, t := range p.PhoneTimings {
		if t.Position != u.Position {
			continue
		}
		switch t.Role {
		case "nucleus":
			if hasMedial {
				result = append(result, provider.SpeechAnchor{SourceMS: pre + math.Min(45, (nasalStart-pre)*.2), TargetMS: t.StartMS - u.NoteStartMS})
			}
		case "offglide":
			result = append(result, provider.SpeechAnchor{SourceMS: nasalStart - math.Min(90, (nasalStart-pre)*.35), TargetMS: t.StartMS - u.NoteStartMS})
		}
	}
	if nasalStart < sourceEnd {
		result = append(result, provider.SpeechAnchor{SourceMS: nasalStart, TargetMS: nasalTarget})
	}
	return result, compound
}
