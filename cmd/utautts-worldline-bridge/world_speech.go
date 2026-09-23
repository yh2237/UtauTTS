package main

import (
	"math"

	"utautts/internal/provider"
)

type worldSpeechMap struct {
	coda                                                                                bool
	separateRelease                                                                     bool
	sourceOnset, targetOnset, sourceFixed, targetFixed, sourceEnd, targetEnd, protected float64
	releaseStart, sourceReleaseStart                                                    float64
}

func worldSpeechAnchors(item unit, duration float64) (worldSpeechMap, bool) {
	if item.Speech == nil || item.Speech.PreserveStopOnly {
		return worldSpeechMap{}, false
	}
	// 解析はoto.offsetちょうどではなく直前のWORLDフレームから始まる。
	shift := math.Max(0, item.OffsetMS) - math.Floor(math.Max(0, item.OffsetMS)/worldFramePeriodMS)*worldFramePeriodMS
	a := worldSpeechMap{sourceOnset: item.Speech.SourceOnsetMS + shift, targetOnset: item.Speech.TargetOnsetMS,
		sourceFixed: item.ConsonantMS + shift, sourceEnd: duration, targetEnd: item.RequiredLengthMS}
	for _, v := range []float64{a.sourceOnset, a.targetOnset, a.sourceEnd, item.OffsetMS, item.Speech.TargetFixedMS} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return worldSpeechMap{}, false
		}
	}
	if item.Speech.CodaRelease {
		a.targetEnd = item.SkipMS + item.LengthMS
		if !math.IsNaN(a.targetEnd) && !math.IsInf(a.targetEnd, 0) && a.sourceOnset >= 0 && a.targetOnset >= 0 && (a.targetOnset > 0 || a.sourceOnset == 0) && a.sourceEnd-a.sourceOnset >= 10 && a.targetEnd-a.targetOnset >= 10 {
			a.coda = true
			a.targetFixed = a.targetEnd
			if item.Speech.ProtectStop {
				a.protected = math.Min(30, math.Min(a.sourceEnd-a.sourceOnset-4, a.targetEnd-a.targetOnset-4))
			}
			// E2a: 末尾の短い解放区間だけを原音と1:1で写し、閉鎖/母音は手前で伸縮する。
			if item.Speech.SeparateRelease && item.Speech.ReleaseMS > 0 {
				releaseMS := item.Speech.ReleaseMS
				if maxLen := a.targetEnd - a.targetOnset - 4; releaseMS > maxLen {
					releaseMS = maxLen
				}
				if releaseMS >= 4 && a.sourceEnd-releaseMS > a.sourceOnset {
					sourceStart := a.sourceEnd - releaseMS
					if item.Speech.SourceTransientMS > 0 {
						// 測定した解放過渡を中心に、その前後を非伸縮で写す。
						sourceStart = item.Speech.SourceTransientMS + shift - 4
					}
					if sourceStart < a.sourceOnset {
						sourceStart = a.sourceOnset
					}
					if sourceStart > a.sourceEnd-releaseMS {
						sourceStart = a.sourceEnd - releaseMS
					}
					a.separateRelease = true
					a.releaseStart = a.targetEnd - releaseMS
					a.sourceReleaseStart = sourceStart
				}
			}
			return a, true
		}
		return worldSpeechMap{}, false
	}
	for _, v := range []float64{a.sourceOnset, a.targetOnset, a.sourceFixed, a.sourceEnd, a.targetEnd, item.OffsetMS} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return worldSpeechMap{}, false
		}
	}
	if a.sourceOnset < 4 || a.targetOnset < 4 || a.sourceFixed-a.sourceOnset < 4 || a.sourceEnd-a.sourceFixed < 20 || a.targetEnd-a.targetOnset < 24 {
		return worldSpeechMap{}, false
	}
	if item.Speech.TargetFixedMS > 0 {
		a.targetFixed = math.Min(a.targetEnd-20, math.Max(a.targetOnset+4, item.Speech.TargetFixedMS))
	} else {
		ratio := (a.targetEnd - a.targetOnset) / (a.sourceEnd - a.sourceOnset)
		transition := (a.sourceFixed - a.sourceOnset) * math.Max(.75, math.Min(1.25, math.Sqrt(ratio)))
		a.targetFixed = math.Min(a.targetEnd-20, a.targetOnset+math.Max(4, transition))
	}
	if item.Speech.ProtectStop {
		a.protected = math.Min(8, math.Min(a.sourceOnset-4, a.targetOnset-4))
	}
	return a, true
}

func (a worldSpeechMap) sourceTime(t float64) float64 {
	t = math.Max(0, math.Min(a.targetEnd, t))
	if a.coda {
		if a.separateRelease {
			if t < a.targetOnset {
				if a.targetOnset <= 0 {
					return 0
				}
				return t * a.sourceOnset / a.targetOnset
			}
			if t < a.releaseStart {
				span := a.releaseStart - a.targetOnset
				if span <= 0 {
					return a.sourceOnset
				}
				return a.sourceOnset + (t-a.targetOnset)*(a.sourceReleaseStart-a.sourceOnset)/span
			}
			return math.Min(a.sourceEnd, a.sourceReleaseStart+(t-a.releaseStart))
		}
		if t < a.targetOnset {
			return t * a.sourceOnset / a.targetOnset
		}
		if t < a.targetOnset+a.protected {
			return a.sourceOnset + t - a.targetOnset
		}
		return a.sourceOnset + a.protected + (t-a.targetOnset-a.protected)*(a.sourceEnd-a.sourceOnset-a.protected)/(a.targetEnd-a.targetOnset-a.protected)
	}
	switch {
	case t < a.targetOnset-a.protected:
		return t * (a.sourceOnset - a.protected) / (a.targetOnset - a.protected)
	case t < a.targetOnset:
		return a.sourceOnset + t - a.targetOnset
	case t < a.targetFixed:
		return a.sourceOnset + (t-a.targetOnset)*(a.sourceFixed-a.sourceOnset)/(a.targetFixed-a.targetOnset)
	default:
		return a.sourceFixed + (t-a.targetFixed)*(a.sourceEnd-a.sourceFixed)/(a.targetEnd-a.targetFixed)
	}
}

// 混合特徴量内の短い有声の連続母音境界だけを平滑化する。キャッシュ済み音源特徴量とターゲットF0曲線は変更しない。
func applyWorldSpeechJoins(input manifest, features *worldFeatures) map[int]provider.WorldSpeechResult {
	report := make(map[int]provider.WorldSpeechResult)
	lastEnd := -1
	for index, item := range input.Units {
		if item.Speech == nil || !item.Speech.VowelJoin || index == 0 {
			continue
		}
		joinMS := item.Speech.TargetJoinMS
		if joinMS <= 0 {
			joinMS = item.Speech.TargetOnsetMS
		}
		centerMS := item.PositionMS + joinMS - item.SkipMS
		if math.IsNaN(centerMS) || math.IsInf(centerMS, 0) {
			continue
		}
		center := int(math.Round(centerMS / worldFramePeriodMS))
		start, end := center-2, center+2
		if start <= lastEnd || start < 0 || end >= features.Frames {
			continue
		}
		leftMS, rightMS := float64(start)*worldFramePeriodMS, float64(end)*worldFramePeriodMS
		previous := input.Units[index-1]
		if previous.PositionMS+previous.LengthMS < leftMS || item.PositionMS > rightMS {
			continue
		}
		blocked := false
		for j, other := range input.Units {
			if j != index && j != index-1 && other.PositionMS < rightMS && other.PositionMS+other.LengthMS > leftMS {
				blocked = true
				break
			}
		}
		if blocked || !smoothWorldVowel(features, start, end) {
			continue
		}
		report[item.Speech.UnitIndex] = provider.WorldSpeechResult{UnitIndex: item.Speech.UnitIndex, JoinApplied: true}
		lastEnd = end
	}
	return report
}

func smoothWorldVowel(f *worldFeatures, start, end int) bool {
	bins := f.FFTSize/2 + 1
	for frame := start; frame <= end; frame++ {
		if f.F0[frame] <= 71 {
			return false
		}
	}
	logs := make([]float64, (end-start+1)*bins)
	for i := range logs {
		v := f.Spectrum[start*bins+i]
		if v <= 1e-12 || math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
		logs[i] = math.Log(v)
	}
	candidate := append([]float64(nil), logs...)
	changeEnergy := 0.0
	for frame := 1; frame < end-start; frame++ {
		for bin := 0; bin < bins; bin++ {
			i := frame*bins + bin
			change := .2 * (logs[i-bins] + logs[i+bins] - 2*logs[i])
			// 局所的なスペクトルパワー変化を1ビンあたり約1.5 dBに制限する。
			changeEnergy += change * change
			candidate[i] += math.Max(-.35, math.Min(.35, change))
		}
	}
	if changeEnergy/float64(len(logs)-2*bins) > .35*.35 {
		return false
	}
	curvature := func(values []float64) float64 {
		sum := 0.0
		for i := bins; i < len(values)-bins; i++ {
			v := values[i-bins] + values[i+bins] - 2*values[i]
			sum += v * v
		}
		return sum / float64(len(values)-2*bins)
	}
	before, after := curvature(logs), curvature(candidate)
	if before < .0009 || after > before*.9 {
		return false
	}
	ap := append([]float64(nil), f.Aperiodicity[start*bins:(end+1)*bins]...)
	for frame := 1; frame < end-start; frame++ {
		for bin := 0; bin < bins; bin++ {
			i := frame*bins + bin
			f.Spectrum[start*bins+i] = math.Exp(candidate[i])
			f.Aperiodicity[start*bins+i] = math.Max(0, math.Min(1, .6*ap[i]+.2*(ap[i-bins]+ap[i+bins])))
		}
	}
	return true
}
