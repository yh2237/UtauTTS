package render

import (
	"math"
	"strings"

	"utautts/internal/plan"
)

// speechRetime keeps the vowel onset at targetOnset. The old two-part mapping
// treats everything before oto.fixed as one interval, so compression can move
// a release burst within that interval. Here onset and stable vowel are separate
// anchors. A short release region is copied for stops, not time-stretched.
func speechRetime(source []float64, targetFrames, sourceOnset, sourceFixed, targetOnset, targetFixed, rate int, stop bool) ([]float64, int, bool) {
	minimum := msToFrames(4, rate)
	if rate <= 0 || minimum < 2 || sourceOnset < minimum || targetOnset < minimum ||
		sourceFixed-sourceOnset < minimum || targetFixed-targetOnset < minimum ||
		len(source)-sourceFixed < msToFrames(20, rate) || targetFrames-targetFixed < msToFrames(20, rate) {
		return nil, targetFixed, false
	}
	// Let stable-vowel stretching carry most of the duration change. The
	// consonant-to-stable-vowel transition may change by at most 25 percent.
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
			// Keep the waveform immediately before the vowel onset intact.
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
	// Reuse the standard overlap-aware stretcher for the transition and vowel.
	tail, err := retimeWithCompressedPrefixUsing(source[sourceOnset-bridge:], targetFrames-targetOnset+bridge,
		sourceFixed-sourceOnset+bridge, targetFixed-targetOnset+bridge, rate, wsolaStretch)
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

func speechStop(p *plan.Plan, unit plan.Unit) bool {
	if unit.Position < 0 || unit.Position >= len(p.Morae) {
		return false
	}
	for _, phone := range p.Morae[unit.Position].Phones {
		if phone.Role == "onset" && strings.Contains(" p py b by t d k ky g gy ", " "+strings.ToLower(phone.Symbol)+" ") {
			return true
		}
	}
	return false
}

// Use the same source-to-output integral as resampleForPitchCurve. Scaling by
// only the first F0 value is insufficient when pitch changes inside the onset.
func speechPitchAnchor(sourceFrames, anchor int, base float64, curve *PitchCurve, startMS, spanMS float64) int {
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
		position += 1 / clampPitchFactor(base*pitchCurveFactorAt(curve, t))
	}
	return int(math.Round(position))
}

// Automatic repair is restricted to a repeated vowel with no intervening
// consonant, coda, pause or transition unit. Explicit boundary-bridge settings
// still use their existing policy. Each renderer evaluates its own repair.
func speechVowelJoin(p *plan.Plan, previous, current renderedUnit) bool {
	if previous.index+1 != current.index || previous.unit.Role != "mora" || current.unit.Role != "mora" || previous.unit.Position+1 != current.unit.Position {
		return false
	}
	if previous.unit.Position < 0 || current.unit.Position >= len(p.Morae) {
		return false
	}
	a, b := p.Morae[previous.unit.Position], p.Morae[current.unit.Position]
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
	for _, unit := range []plan.Unit{previous.unit, current.unit} {
		if unit.SpeechProfile == nil || !unit.SpeechProfile.Applied {
			return false
		}
	}
	return true
}
