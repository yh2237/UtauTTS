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
	singleCVDefaultVowelOverlapMS = 18.0
	singleCVSameVowelOverlapMS    = 24.0
	vcvMinimumPreutteranceMS      = 48
	vcvMaximumPreutteranceMS      = 150
	vcvMinimumVowelTailMS         = 45
	vcvVowelTailRatio             = 0.30
)

// C3a: 音源実測に対する過度な伸縮をモーラ内で有界にする。
const (
	// stretchAdaptMaxRatioは母音側（fixed以降）の許容伸縮上限。
	stretchAdaptMaxRatio = 1.3
	// stretchAdaptMinTailRatioは母音側に残す最小比率。子音側を伸ばしすぎない。
	stretchAdaptMinTailRatio = 0.35
	// stretchAdaptMinTailMSは母音側に残す最小長。
	stretchAdaptMinTailMS = 40.0
	// stretchAdaptStrengthLimitは補正強度の上限。
	stretchAdaptStrengthLimit = 2.0
)

// adaptStretchTimingはSpeechProfileの実測長に対し母音側の伸縮が過大なとき、
// 総長を変えずにfixed境界を後ろへずらして母音の伸びを抑える。無効時と負値強度は恒等。
func adaptStretchTiming(unit plan.Unit, timing effectiveTiming, releaseMS float64, enabled bool, strength float64) effectiveTiming {
	if !enabled || unit.Silent || unit.Role != "mora" || unit.DurationMS <= 0 {
		return timing
	}
	profile := unit.SpeechProfile
	if profile == nil || !profile.Applied || profile.TrimmedLengthMS <= 0 {
		return timing
	}
	if strength < 0 {
		return timing
	}
	if strength == 0 {
		strength = 1
	}
	if strength > stretchAdaptStrengthLimit {
		strength = stretchAdaptStrengthLimit
	}
	// 総長(preutterance + duration + release)は変えない。
	targetTotal := timing.preutteranceMS + unit.DurationMS + releaseMS
	sourceFixed := math.Max(0, unit.ConsonantMS)
	if sourceFixed > profile.TrimmedLengthMS {
		sourceFixed = profile.TrimmedLengthMS
	}
	sourceTail := profile.TrimmedLengthMS - sourceFixed
	if sourceTail <= 0 {
		return timing
	}
	targetTail := targetTotal - timing.consonantMS
	if targetTail <= 0 {
		return timing
	}
	if targetTail/sourceTail <= stretchAdaptMaxRatio {
		return timing
	}
	// 超過分をfixed側へ移し、母音側の伸びを上限へ近づける。
	allowedTail := sourceTail * stretchAdaptMaxRatio
	targetFixed := targetTotal - allowedTail
	targetFixed = timing.consonantMS + (targetFixed-timing.consonantMS)*strength
	// 母音側の最小長を確保して子音の過度な伸長を防ぐ。
	minimumTail := math.Max(stretchAdaptMinTailMS, targetTotal*stretchAdaptMinTailRatio)
	if maximumFixed := targetTotal - minimumTail; targetFixed > maximumFixed {
		targetFixed = maximumFixed
	}
	if targetFixed <= timing.consonantMS {
		return timing
	}
	timing.consonantMS = targetFixed
	timing.stretchAdapted = true
	timing.stretchLimitReason = "source-length-bound"
	return timing
}

// onsetOverlapClassはoverlap調整のための子音クラスを返す。
func onsetOverlapClass(onset string) string {
	switch strings.ToLower(strings.TrimSpace(onset)) {
	case "p", "b", "t", "d", "k", "g", "q", "py", "by", "ty", "dy", "ky", "gy",
		"ch", "jh", "ts", "dz", "c", "j":
		return "stop"
	case "m", "n", "ny", "r", "l", "w", "y":
		return "sonorant"
	}
	return ""
}

// onsetOverlapMSは子音クラスに応じてoverlapを調整する。
// 破裂・破擦音は重ねを小さくし、鼻音・流音は前の母音を重ねる。
func onsetOverlapMS(onset string, preutterance, overlap float64) float64 {
	switch onsetOverlapClass(onset) {
	case "stop":
		if limit := preutterance * 0.1; overlap > limit {
			overlap = limit
		}
	case "sonorant":
		if target := preutterance * 0.5; overlap < target {
			overlap = target
		}
	}
	return math.Max(0, math.Min(overlap, preutterance))
}

func normalizePlanTiming(synthesisPlan *plan.Plan, unit plan.Unit, releaseMS float64) effectiveTiming {
	timing := normalizeTiming(unit, releaseMS)
	if synthesisPlan == nil || unit.Silent || unit.Role != "mora" {
		return timing
	}
	if synthesisPlan.SingleCV {
		return normalizeSingleCVTiming(synthesisPlan, unit, timing, releaseMS)
	}
	if strings.EqualFold(strings.TrimSpace(unit.AliasKind), "VCV") {
		timing = normalizeVCVTiming(unit, timing, releaseMS)
	}
	timing.overlapMS = onsetOverlapMS(singleCVOnset(synthesisPlan, unit), timing.preutteranceMS, timing.overlapMS)
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

// brokenVCVTimingはoto.iniの位置が境界として成立しないVCVを判定する。
func brokenVCVTiming(unit plan.Unit) bool {
	if unit.PreutteranceMS < 0 || unit.ConsonantMS < 0 || unit.OverlapMS < 0 {
		return true
	}
	if unit.OverlapMS > unit.PreutteranceMS || unit.ConsonantMS < unit.PreutteranceMS {
		return true
	}
	if profile := unit.SpeechProfile; profile != nil && profile.Applied && profile.TrimmedLengthMS > 0 &&
		unit.PreutteranceMS > profile.TrimmedLengthMS {
		return true
	}
	return false
}

// preservedVCVTimingはoto.iniの位置をそのまま使う。
func preservedVCVTiming(unit plan.Unit) effectiveTiming {
	preutterance := math.Max(0, unit.PreutteranceMS)
	overlap := math.Max(0, unit.OverlapMS)
	if overlap > preutterance {
		overlap = preutterance
	}
	return effectiveTiming{
		preutteranceMS: preutterance,
		consonantMS:    math.Max(0, unit.ConsonantMS),
		overlapMS:      overlap,
		scale:          1,
	}
}

// normalizeVCVTimingは壊れたVCV境界だけを補正する。正常なoto.iniの位置はそのまま使う。
func normalizeVCVTiming(unit plan.Unit, timing effectiveTiming, releaseMS float64) effectiveTiming {
	if !brokenVCVTiming(unit) {
		return preservedVCVTiming(unit)
	}
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
		return 18
	case "ch", "jh", "ts", "dz", "c", "j":
		return 24
	case "s", "sh", "f", "h", "z", "zh", "x", "v":
		return 32
	case "m", "n", "ny", "r", "l", "w", "y":
		return 40
	default:
		return 36
	}
}

func singleCVWorldOverlapMS(synthesisPlan *plan.Plan, unit plan.Unit, preutteranceMS float64) float64 {
	overlap := math.Max(0, unit.OverlapMS)
	overlap = math.Min(overlap, singleCVOnsetFadeInMS(synthesisPlan, unit))
	if singleCVVowelBoundaryWithoutOnset(synthesisPlan, unit) {
		// 単独母音はotoのoverlap=0でも前の母音と短く重ねる。
		if overlap <= 0 {
			overlap = singleCVDefaultVowelOverlapMS
			if sameSingleCVVowel(synthesisPlan, unit.Position) {
				overlap = singleCVSameVowelOverlapMS
			}
		}
		return math.Min(overlap, singleCVOnsetFadeInMS(synthesisPlan, unit))
	}
	return math.Min(overlap, math.Max(0, preutteranceMS))
}

func singleCVVowelBoundaryWithoutOnset(synthesisPlan *plan.Plan, unit plan.Unit) bool {
	if synthesisPlan == nil || singleCVOnset(synthesisPlan, unit) != "" {
		return false
	}
	return singleCVMoraBoundaryEligible(synthesisPlan, unit.Position)
}

func sameSingleCVVowel(synthesisPlan *plan.Plan, position int) bool {
	if synthesisPlan == nil || position <= 0 || position >= len(synthesisPlan.Morae) {
		return false
	}
	return synthesisPlan.Morae[position-1].Vowel == synthesisPlan.Morae[position].Vowel
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
