package base

import (
	"math"
	"strings"

	"utautts/internal/plan"
)

const (
	// 単独音には先行母音遷移がないため子音開始を短くし母音末尾を確保する
	SingleCVPreutteranceRatio     = 0.60
	SingleCVMinimumPreutteranceMS = 32
	SingleCVMaximumPreutteranceMS = 110
	SingleCVMinimumVowelTailMS    = 35
	SingleCVVowelTailRatio        = 0.35
	SingleCVDefaultVowelOverlapMS = 18.0
	SingleCVSameVowelOverlapMS    = 24.0
	VCVMinimumPreutteranceMS      = 48
	VCVMaximumPreutteranceMS      = 150
	VCVMinimumVowelTailMS         = 45
	VCVVowelTailRatio             = 0.30
)

// C3a: 音源実測に対する過度な伸縮をモーラ内で有界にする。
const (
	// StretchAdaptMaxRatioは母音側（fixed以降）の許容伸縮上限。
	StretchAdaptMaxRatio = 1.3
	// StretchAdaptMinTailRatioは母音側に残す最小比率。子音側を伸ばしすぎない。
	StretchAdaptMinTailRatio = 0.35
	// StretchAdaptMinTailMSは母音側に残す最小長。
	StretchAdaptMinTailMS = 40.0
	// StretchAdaptStrengthLimitは補正強度の上限。
	StretchAdaptStrengthLimit = 2.0
)

// AdaptStretchTimingはSpeechProfileの実測長に対し母音側の伸縮が過大なとき、
// 総長を変えずにfixed境界を後ろへずらして母音の伸びを抑える。無効時と負値強度は恒等。
func AdaptStretchTiming(unit plan.Unit, timing EffectiveTiming, releaseMS float64, enabled bool, strength float64) EffectiveTiming {
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
	if strength > StretchAdaptStrengthLimit {
		strength = StretchAdaptStrengthLimit
	}
	// 総長(preutterance + duration + release)は変えない。
	targetTotal := timing.PreutteranceMS + unit.DurationMS + releaseMS
	sourceFixed := math.Max(0, unit.ConsonantMS)
	if sourceFixed > profile.TrimmedLengthMS {
		sourceFixed = profile.TrimmedLengthMS
	}
	sourceTail := profile.TrimmedLengthMS - sourceFixed
	if sourceTail <= 0 {
		return timing
	}
	targetTail := targetTotal - timing.ConsonantMS
	if targetTail <= 0 {
		return timing
	}
	if targetTail/sourceTail <= StretchAdaptMaxRatio {
		return timing
	}
	// 超過分をfixed側へ移し、母音側の伸びを上限へ近づける。
	allowedTail := sourceTail * StretchAdaptMaxRatio
	targetFixed := targetTotal - allowedTail
	targetFixed = timing.ConsonantMS + (targetFixed-timing.ConsonantMS)*strength
	// 母音側の最小長を確保して子音の過度な伸長を防ぐ。
	minimumTail := math.Max(StretchAdaptMinTailMS, targetTotal*StretchAdaptMinTailRatio)
	if maximumFixed := targetTotal - minimumTail; targetFixed > maximumFixed {
		targetFixed = maximumFixed
	}
	if targetFixed <= timing.ConsonantMS {
		return timing
	}
	timing.ConsonantMS = targetFixed
	timing.StretchAdapted = true
	timing.StretchLimitReason = "source-length-bound"
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

// OnsetOverlapMSは子音クラスに応じてoverlapを調整する。
// 破裂・破擦音は重ねを小さくし、鼻音・流音は前の母音を重ねる。
func OnsetOverlapMS(onset string, preutterance, overlap float64) float64 {
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

const (
	// CodaBoundaryMinTailMSはcoda境界で語末子音の末尾に残す最低長。
	CodaBoundaryMinTailMS = 30.0
	// CodaBoundaryMaxOverlapMSはcoda境界で次onsetが重ねる上限。
	CodaBoundaryMaxOverlapMS = 15.0
)

// CodaBoundaryOverlapMSは語末子音を持つユニット境界で、次onsetの食い込みと
// 重なりを制限して語末子音の末尾を残す。OnsetOverlapMSと同じく有界にクランプする。
// 適用したかどうかも返す。
func CodaBoundaryOverlapMS(previousDuration, preutterance, overlap float64) (float64, float64, bool) {
	limited := false
	if previousDuration > CodaBoundaryMinTailMS {
		if limit := previousDuration - CodaBoundaryMinTailMS; preutterance > limit {
			preutterance = limit
			limited = true
		}
	}
	if overlap > CodaBoundaryMaxOverlapMS {
		overlap = CodaBoundaryMaxOverlapMS
		limited = true
	}
	if overlap > preutterance {
		overlap = preutterance
	}
	return math.Max(0, preutterance), math.Max(0, overlap), limited
}

// NormalizePlanTimingはPlan形式に応じて実効タイミングを補正する。
func NormalizePlanTiming(synthesisPlan *plan.Plan, unit plan.Unit, releaseMS float64) EffectiveTiming {
	timing := NormalizeTiming(unit, releaseMS)
	if synthesisPlan == nil || unit.Silent || unit.Role != "mora" {
		return timing
	}
	if synthesisPlan.SingleCV {
		return normalizeSingleCVTiming(synthesisPlan, unit, timing, releaseMS)
	}
	if strings.EqualFold(strings.TrimSpace(unit.AliasKind), "VCV") {
		timing = normalizeVCVTiming(unit, timing, releaseMS)
	}
	timing.OverlapMS = OnsetOverlapMS(SingleCVOnset(synthesisPlan, unit), timing.PreutteranceMS, timing.OverlapMS)
	return timing
}

// NormalizedPhoneTimingUnitsはwaveformと同じ補正値をphrase timingへ渡す。
// 形式別補正が不要な場合はPlanのsliceをそのまま返す。
func NormalizedPhoneTimingUnits(synthesisPlan *plan.Plan, releaseMS float64) []plan.Unit {
	if synthesisPlan == nil {
		return nil
	}
	needsCopy := synthesisPlan.SingleCV
	if !needsCopy {
		for _, unit := range synthesisPlan.Units {
			if IsVCVUnit(unit) {
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
		if unit.Silent || unit.Role != "mora" || (!synthesisPlan.SingleCV && !IsVCVUnit(unit)) {
			continue
		}
		timing := NormalizePlanTiming(synthesisPlan, unit, releaseMS)
		result[index].PreutteranceMS = timing.PreutteranceMS
		result[index].OverlapMS = timing.OverlapMS
		result[index].ConsonantMS = timing.ConsonantMS
	}
	return result
}

// IsVCVUnitはoto.iniのVCV形式かを返す。
func IsVCVUnit(unit plan.Unit) bool {
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
func preservedVCVTiming(unit plan.Unit) EffectiveTiming {
	preutterance := math.Max(0, unit.PreutteranceMS)
	overlap := math.Max(0, unit.OverlapMS)
	if overlap > preutterance {
		overlap = preutterance
	}
	return EffectiveTiming{
		PreutteranceMS: preutterance,
		ConsonantMS:    math.Max(0, unit.ConsonantMS),
		OverlapMS:      overlap,
		Scale:          1,
	}
}

// normalizeVCVTimingは壊れたVCV境界だけを補正する。正常なoto.iniの位置はそのまま使う。
func normalizeVCVTiming(unit plan.Unit, timing EffectiveTiming, releaseMS float64) EffectiveTiming {
	if !brokenVCVTiming(unit) {
		return preservedVCVTiming(unit)
	}
	duration := math.Max(1, unit.DurationMS)
	releaseMS = math.Max(0, releaseMS)
	rawPreutterance := math.Max(0, unit.PreutteranceMS)
	preutterance := math.Max(0, timing.PreutteranceMS)
	overlap := math.Max(0, timing.OverlapMS)
	warnings := make([]string, 0, 3)
	changed := false

	maxPreutterance := math.Max(VCVMinimumPreutteranceMS, math.Min(VCVMaximumPreutteranceMS, duration*0.75))
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
	minimumTail := math.Max(VCVMinimumVowelTailMS, duration*VCVVowelTailRatio)
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

	scale := timing.Scale
	if rawPreutterance > 0 {
		scale = math.Min(scale, preutterance/rawPreutterance)
	}
	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		scale = 1
	}
	return EffectiveTiming{
		PreutteranceMS: preutterance,
		ConsonantMS:    targetFixed,
		OverlapMS:      overlap,
		Scale:          scale,
		CVApplied:      changed,
		CVWarnings:     uniqueTimingWarnings(warnings),
	}
}

func normalizeSingleCVTiming(synthesisPlan *plan.Plan, unit plan.Unit, timing EffectiveTiming, releaseMS float64) EffectiveTiming {
	duration := math.Max(1, unit.DurationMS)
	releaseMS = math.Max(0, releaseMS)
	rawPreutterance := math.Max(0, unit.PreutteranceMS)
	rawConsonant := math.Max(0, unit.ConsonantMS)
	preutterance := math.Max(0, timing.PreutteranceMS)
	overlap := math.Max(0, timing.OverlapMS)
	warnings := make([]string, 0, 4)
	changed := false

	maxPreutterance := math.Max(SingleCVMinimumPreutteranceMS, math.Min(SingleCVMaximumPreutteranceMS, duration*SingleCVPreutteranceRatio))
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
	minimumTail := math.Max(SingleCVMinimumVowelTailMS, duration*SingleCVVowelTailRatio)
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
	scale := timing.Scale
	if rawPreutterance > 0 {
		scale = math.Min(scale, preutterance/rawPreutterance)
	}
	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		scale = 1
	}
	return EffectiveTiming{
		PreutteranceMS: preutterance,
		ConsonantMS:    targetFixed,
		OverlapMS:      overlap,
		Scale:          scale,
		CVApplied:      changed,
		CVWarnings:     uniqueTimingWarnings(warnings),
	}
}

func singleCVOnsetFadeInMS(synthesisPlan *plan.Plan, unit plan.Unit) float64 {
	onset := SingleCVOnset(synthesisPlan, unit)
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

// SingleCVWorldOverlapMSは単独音のWORLD向けoverlapを返す。
func SingleCVWorldOverlapMS(synthesisPlan *plan.Plan, unit plan.Unit, preutteranceMS float64) float64 {
	overlap := math.Max(0, unit.OverlapMS)
	overlap = math.Min(overlap, singleCVOnsetFadeInMS(synthesisPlan, unit))
	if singleCVVowelBoundaryWithoutOnset(synthesisPlan, unit) {
		// 単独母音はotoのoverlap=0でも前の母音と短く重ねる。
		if overlap <= 0 {
			overlap = SingleCVDefaultVowelOverlapMS
			if sameSingleCVVowel(synthesisPlan, unit.Position) {
				overlap = SingleCVSameVowelOverlapMS
			}
		}
		return math.Min(overlap, singleCVOnsetFadeInMS(synthesisPlan, unit))
	}
	return math.Min(overlap, math.Max(0, preutteranceMS))
}

func singleCVVowelBoundaryWithoutOnset(synthesisPlan *plan.Plan, unit plan.Unit) bool {
	if synthesisPlan == nil || SingleCVOnset(synthesisPlan, unit) != "" {
		return false
	}
	return SingleCVMoraBoundaryEligible(synthesisPlan, unit.Position)
}

func sameSingleCVVowel(synthesisPlan *plan.Plan, position int) bool {
	if synthesisPlan == nil || position <= 0 || position >= len(synthesisPlan.Morae) {
		return false
	}
	return synthesisPlan.Morae[position-1].Vowel == synthesisPlan.Morae[position].Vowel
}

// SingleCVOnsetはモーラの子音onsetを返す。
func SingleCVOnset(synthesisPlan *plan.Plan, unit plan.Unit) string {
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

// SingleCVBoundaryEligibleは単独音の境界補正対象かを返す。
func SingleCVBoundaryEligible(synthesisPlan *plan.Plan, previous, current RenderedUnit) bool {
	if synthesisPlan == nil || !synthesisPlan.SingleCV || previous.Index+1 != current.Index {
		return false
	}
	if previous.Unit.Role != "mora" || current.Unit.Role != "mora" || previous.Unit.Position+1 != current.Unit.Position {
		return false
	}
	if previous.Unit.Silent || current.Unit.Silent || current.Unit.Position < 0 || current.Unit.Position >= len(synthesisPlan.Morae) {
		return false
	}
	if len(previous.Unit.CodaPhones) > 0 {
		return false
	}
	return SingleCVMoraBoundaryEligible(synthesisPlan, current.Unit.Position)
}

// SingleCVMoraBoundaryEligibleは単独音のモーラ境界が補正対象かを返す。
func SingleCVMoraBoundaryEligible(synthesisPlan *plan.Plan, position int) bool {
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

// SingleCVProtectedOnsetは破裂・破擦音のonsetかを返す。
func SingleCVProtectedOnset(synthesisPlan *plan.Plan, unit plan.Unit) bool {
	symbol := strings.ToLower(strings.TrimSpace(SingleCVOnset(synthesisPlan, unit)))
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

// NormalizeTimingはoto.iniの値を有界な実効タイミングへ補正する。
func NormalizeTiming(unit plan.Unit, releaseMS float64) EffectiveTiming {
	preutterance := math.Max(0, unit.PreutteranceMS)
	overlap := unit.OverlapMS
	consonant := math.Max(0, unit.ConsonantMS)
	scale := 1.0

	if preutterance > math.Max(120, unit.DurationMS*1.5) {
		effectivePreutterance := math.Max(80, unit.DurationMS*0.75)
		scale = effectivePreutterance / preutterance
		preutterance = effectivePreutterance
		if overlap > 0 {
			overlap *= scale
		}
		consonant *= scale
	}
	overlap = math.Min(overlap, preutterance)

	if scale < 1 {
		targetMS := preutterance + unit.DurationMS + releaseMS
		minimumTailMS := releaseMS + math.Max(40, unit.DurationMS*0.35)
		consonant = math.Min(consonant, math.Max(0, targetMS-minimumTailMS))
	}
	return EffectiveTiming{PreutteranceMS: preutterance, ConsonantMS: consonant, OverlapMS: overlap, Scale: scale}
}

// LimitLeadingPreutteranceは0を自動扱いにし、指定時だけ文頭側の保持区間を制限する。
func LimitLeadingPreutterance(required, maximum float64) float64 {
	if maximum <= 0 {
		return required
	}
	return math.Min(required, maximum)
}

// FadeInDurationMSは前後のゲインが同時に0にならない重なりを返す。
func FadeInDurationMS(timing EffectiveTiming) float64 {
	// 特殊なoto設定でも前後のゲインが同時に0にならないよう重なりを確保する。
	return math.Max(6, timing.PreutteranceMS-timing.OverlapMS)
}
