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
	vcvMinimumPreutteranceMS      = 48
	vcvMaximumPreutteranceMS      = 150
	vcvMinimumVowelTailMS         = 45
	vcvVowelTailRatio             = 0.30
)

func normalizePlanTiming(synthesisPlan *plan.Plan, unit plan.Unit, releaseMS float64) effectiveTiming {
	timing := normalizeTiming(unit, releaseMS)
	if synthesisPlan == nil || unit.Silent || unit.Role != "mora" {
		return timing
	}
	if synthesisPlan.SingleCV {
		return normalizeSingleCVTiming(synthesisPlan, unit, timing, releaseMS)
	}
	if strings.EqualFold(strings.TrimSpace(unit.AliasKind), "VCV") {
		return normalizeVCVTiming(unit, timing, releaseMS)
	}
	return timing
}

// normalizedPhoneTimingUnitsはwaveformと同じ補正値をphrase timingへ渡す。
// 形式別補正が不要な場合はPlanのsliceをそのまま返す。
func normalizedPhoneTimingUnits(synthesisPlan *plan.Plan, releaseMS float64) []plan.Unit {
	if synthesisPlan == nil {
		return nil
	}
	needsCopy := synthesisPlan.SingleCV
	if !needsCopy {
		for _, unit := range synthesisPlan.Units {
			if isVCVUnit(unit) {
				needsCopy = true
				break
			}
		}
	}
	if !needsCopy {
		return synthesisPlan.Units
	}
	result := append([]plan.Unit(nil), synthesisPlan.Units...)
	for index := range result {
		unit := result[index]
		if unit.Silent || unit.Role != "mora" || (!synthesisPlan.SingleCV && !isVCVUnit(unit)) {
			continue
		}
		timing := normalizePlanTiming(synthesisPlan, unit, releaseMS)
		result[index].PreutteranceMS = timing.preutteranceMS
		result[index].OverlapMS = timing.overlapMS
		result[index].ConsonantMS = timing.consonantMS
	}
	return result
}

func isVCVUnit(unit plan.Unit) bool {
	return strings.EqualFold(strings.TrimSpace(unit.AliasKind), "VCV")
}

// normalizeVCVTimingは長い録音のVCV境界を短いモーラへ収める。
// fixedをそのまま使うと対象モーラの母音がほとんど残らないことがある。
func normalizeVCVTiming(unit plan.Unit, timing effectiveTiming, releaseMS float64) effectiveTiming {
	duration := math.Max(1, unit.DurationMS)
	releaseMS = math.Max(0, releaseMS)
	rawPreutterance := math.Max(0, unit.PreutteranceMS)
	preutterance := math.Max(0, timing.preutteranceMS)
	overlap := math.Max(0, timing.overlapMS)
	warnings := make([]string, 0, 3)
	changed := false

	maxPreutterance := math.Max(vcvMinimumPreutteranceMS, math.Min(vcvMaximumPreutteranceMS, duration*0.75))
	if preutterance > maxPreutterance {
		preutterance = maxPreutterance
		changed = true
		warnings = append(warnings, "vcv-preutterance-clamped")
	}
	if rawPreutterance > 0 && preutterance < rawPreutterance && overlap > 0 {
		overlap *= preutterance / rawPreutterance
	}
	overlap = math.Min(overlap, preutterance)

	transition := math.Max(0, unit.ConsonantMS-rawPreutterance)
	if profile := unit.SpeechProfile; profile != nil && profile.Applied {
		// otoの位置ずれをstable startで補い、破裂音の長さは範囲内に保つ。
		if profile.StableStartMS > rawPreutterance {
			transition = math.Max(transition, profile.StableStartMS-rawPreutterance)
		}
		if profile.TrimmedLengthMS > 0 {
			ratio := (duration + releaseMS) / profile.TrimmedLengthMS
			ratio = math.Max(0.75, math.Min(1.25, math.Sqrt(ratio)))
			transition *= ratio
		}
	}
	targetFixed := preutterance + transition
	targetMS := preutterance + duration + releaseMS
	minimumTail := math.Max(vcvMinimumVowelTailMS, duration*vcvVowelTailRatio)
	maximumFixed := math.Max(preutterance, targetMS-minimumTail)
	if targetFixed > maximumFixed {
		targetFixed = maximumFixed
		changed = true
		warnings = append(warnings, "vcv-vowel-tail-preserved")
	}
	if targetFixed < preutterance {
		targetFixed = preutterance
	}
	if math.Abs(targetFixed-unit.ConsonantMS) > 0.5 {
		changed = true
		warnings = append(warnings, "vcv-fixed-retimed")
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
