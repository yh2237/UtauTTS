package render

import "utautts/internal/plan"

// 主母音の音高を基準にし VCや語末子音による輪郭の重複を防ぐ。
func speechReferencePitchFactors(p *plan.Plan, pitches []float64, fallback float64) ([]float64, float64) {
	var mains []float64
	for i, u := range p.Units {
		if u.Role == "mora" && !u.Silent && pitches[i] > 0 {
			mains = append(mains, pitches[i])
		}
	}
	reference := medianFloat(mains)
	if reference <= 0 {
		reference = fallback
	}
	if reference <= 0 {
		reference = 220
	}
	factors := make([]float64, len(pitches))
	for i, source := range pitches {
		if source <= 0 {
			source = reference
		}
		factors[i] = reference / source * effectiveUnitPitchFactor(p.Units[i], true)
	}
	return factors, reference
}
