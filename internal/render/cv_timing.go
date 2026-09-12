package render

import (
	"math"
	"strings"

	"utautts/internal/plan"
)

const (
	// 単独音には先行母音遷移がないため子音開始を短くし母音末尾を確保する
	singleCVPreutteranceRatio     = 0.60
	singleCVMinimumPreutteranceMS = 32
	singleCVMaximumPreutteranceMS = 110
	singleCVMinimumVowelTailMS    = 35
	singleCVVowelTailRatio        = 0.35
)

func normalizePlanTiming(synthesisPlan *plan.Plan, unit plan.Unit, releaseMS float64) effectiveTiming {
	timing := normalizeTiming(unit, releaseMS)
	if synthesisPlan == nil || !synthesisPlan.SingleCV || unit.Silent || unit.Role != "mora" {
		return timing
	}
	return normalizeSingleCVTiming(synthesisPlan, unit, timing, releaseMS)
}

func normalizeSingleCVTiming(synthesisPlan *plan.Plan, unit plan.Unit, timing effectiveTiming, releaseMS float64) effectiveTiming {
	duration := math.Max(1, unit.DurationMS)
	releaseMS = math.Max(0, releaseMS)
	rawPreutterance := math.Max(0, unit.PreutteranceMS)
	rawConsonant := math.Max(0, unit.ConsonantMS)
	preutterance := math.Max(0, timing.preutteranceMS)
	overlap := math.Max(0, timing.overlapMS)
	warnings := make([]string, 0, 4)
	changed := false

	maxPreutterance := math.Max(singleCVMinimumPreutteranceMS, math.Min(singleCVMaximumPreutteranceMS, duration*singleCVPreutteranceRatio))
	if preutterance > maxPreutterance {
		preutterance = maxPreutterance
		changed = true
		warnings = append(warnings, "preutterance-clamped")
	}

	// CからVへの遷移は保ちつつ長すぎる先頭部分を圧縮し残りを母音へ回す
	transition := math.Max(0, rawConsonant-rawPreutterance)
	if profile := unit.SpeechProfile; profile != nil && profile.TrimmedLengthMS > 0 && transition > 0 {
		ratio := (duration + releaseMS) / profile.TrimmedLengthMS
		ratio = math.Max(0.5, math.Min(1.15, ratio))
		transition *= math.Sqrt(ratio)
	}
	if transition > 0 {
		transition = math.Min(transition, duration*0.55)
	}
	targetFixed := preutterance + transition
	targetMS := preutterance + duration + releaseMS
	minimumTail := math.Max(singleCVMinimumVowelTailMS, duration*singleCVVowelTailRatio)
	maximumFixed := math.Max(preutterance, targetMS-minimumTail)
	if targetFixed > maximumFixed {
		targetFixed = maximumFixed
		changed = true
		warnings = append(warnings, "vowel-tail-preserved")
	}
	if targetFixed < preutterance {
		targetFixed = preutterance
	}

	// 単独音の子音は早く聞こえる必要があるため子音種別ごとに短くフェードする
	if preutterance > 0 {
		fadeIn := math.Min(preutterance, singleCVOnsetFadeInMS(synthesisPlan, unit))
		if fadeIn < preutterance-overlap {
			overlap = preutterance - fadeIn
			changed = true
			warnings = append(warnings, "onset-fade-shortened")
		}
	}
	overlap = math.Max(0, math.Min(overlap, preutterance))
	if math.Abs(targetFixed-unit.ConsonantMS) > 0.5 {
		changed = true
	}
	scale := timing.scale
	if rawPreutterance > 0 {
		scale = math.Min(scale, preutterance/rawPreutterance)
	}
	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		scale = 1
	}
	return effectiveTiming{
		preutteranceMS: preutterance,
		consonantMS:    targetFixed,
		overlapMS:      overlap,
		scale:          scale,
		cvApplied:      changed,
		cvWarnings:     uniqueTimingWarnings(warnings),
	}
}

func singleCVOnsetFadeInMS(synthesisPlan *plan.Plan, unit plan.Unit) float64 {
	onset := singleCVOnset(synthesisPlan, unit)
	switch strings.ToLower(strings.TrimSpace(onset)) {
	case "p", "b", "t", "d", "k", "g", "q", "py", "by", "ty", "dy", "ky", "gy", "cl":
		return 12
	case "ch", "jh", "ts", "dz", "c", "j":
		return 17
	case "s", "sh", "f", "h", "z", "zh", "x", "v":
		return 24
	case "m", "n", "ny", "r", "l", "w", "y":
		return 28
	default:
		return 20
	}
}

func singleCVWorldOverlapMS(synthesisPlan *plan.Plan, unit plan.Unit, preutteranceMS float64) float64 {
	overlap := math.Max(0, unit.OverlapMS)
	overlap = math.Min(overlap, singleCVOnsetFadeInMS(synthesisPlan, unit))
	return math.Min(overlap, math.Max(0, preutteranceMS))
}

func singleCVOnset(synthesisPlan *plan.Plan, unit plan.Unit) string {
	if synthesisPlan == nil || unit.Position < 0 || unit.Position >= len(synthesisPlan.Morae) {
		return ""
	}
	mora := synthesisPlan.Morae[unit.Position]
	if mora.Consonant != "" {
		return mora.Consonant
	}
	for _, phone := range mora.Phones {
		if phone.Role == "onset" {
			return phone.Symbol
		}
	}
	return ""
}

func singleCVBoundaryEligible(synthesisPlan *plan.Plan, previous, current renderedUnit) bool {
	if synthesisPlan == nil || !synthesisPlan.SingleCV || previous.index+1 != current.index {
		return false
	}
	if previous.unit.Role != "mora" || current.unit.Role != "mora" || previous.unit.Position+1 != current.unit.Position {
		return false
	}
	if previous.unit.Silent || current.unit.Silent || current.unit.Position < 0 || current.unit.Position >= len(synthesisPlan.Morae) {
		return false
	}
	if len(previous.unit.CodaPhones) > 0 {
		return false
	}
	return singleCVMoraBoundaryEligible(synthesisPlan, current.unit.Position)
}

func singleCVMoraBoundaryEligible(synthesisPlan *plan.Plan, position int) bool {
	if synthesisPlan == nil || position <= 0 || position >= len(synthesisPlan.Morae) {
		return false
	}
	previous := synthesisPlan.Morae[position-1]
	mora := synthesisPlan.Morae[position]
	if previous.Pause || mora.Pause || previous.Vowel == "" || previous.Vowel == "cl" || previous.Vowel == "n" ||
		mora.Vowel == "" || mora.Vowel == "cl" || mora.Vowel == "n" {
		return false
	}
	for _, phone := range previous.Phones {
		if phone.Role == "coda" {
			return false
		}
	}
	return true
}

func singleCVProtectedOnset(synthesisPlan *plan.Plan, unit plan.Unit) bool {
	symbol := strings.ToLower(strings.TrimSpace(singleCVOnset(synthesisPlan, unit)))
	switch symbol {
	case "p", "b", "t", "d", "k", "g", "q", "py", "by", "ty", "dy", "ky", "gy", "ch", "jh", "ts", "dz", "c", "j":
		return true
	default:
		return false
	}
}

func uniqueTimingWarnings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok || strings.TrimSpace(value) == "" {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
