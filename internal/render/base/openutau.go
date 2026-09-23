package base

import (
	"math"

	"utautts/internal/plan"
	"utautts/internal/provider"
)

// OpenUtauPhoneTimingsはOpenUTAU互換の音素時間を作る。
func OpenUtauPhoneTimings(units []plan.Unit, cvvcTiming string) ([]OpenUtauPhoneTiming, float64) {
	return OpenUtauPhoneTimingsWithCoda(units, cvvcTiming, false)
}

// OpenUtauPhoneTimingsWithCodaは語末子音を持つ境界を保護して音素時間を作る。
func OpenUtauPhoneTimingsWithCoda(units []plan.Unit, cvvcTiming string, protectCoda bool) ([]OpenUtauPhoneTiming, float64) {
	result := make([]OpenUtauPhoneTiming, len(units))
	previous := -1
	first := -1
	for index, unit := range units {
		if unit.Silent {
			continue
		}
		if first < 0 && unit.Role != "transition" {
			first = index
		}
		autoPreutter := unit.PreutteranceMS
		autoOverlap := unit.OverlapMS
		adjacent := false
		codaLimited := false
		if previous >= 0 {
			previousUnit := units[previous]
			gapMS := unit.NoteStartMS - (previousUnit.NoteStartMS + previousUnit.DurationMS)
			previousDuration := previousUnit.DurationMS
			maxPreutter := autoPreutter
			if gapMS <= 0 {
				adjacent = true
				if autoOverlap > 0 && autoPreutter-autoOverlap > previousDuration*0.5 {
					maxPreutter = previousDuration * 0.5 / (autoPreutter - autoOverlap) * autoPreutter
				} else if autoOverlap <= 0 {
					maxPreutter = math.Min(maxPreutter, previousDuration*0.9)
				}
				maxPreutter = math.Min(maxPreutter, previousDuration)
				if result[previous].Preutter < 5 {
					maxPreutter = math.Min(maxPreutter, previousDuration+result[previous].Preutter-5)
				}
			} else if gapMS < autoPreutter {
				maxPreutter = gapMS
			}
			if autoPreutter > maxPreutter && autoPreutter > 0 {
				ratio := maxPreutter / autoPreutter
				autoPreutter = maxPreutter
				autoOverlap *= ratio
			}
			if autoOverlap < 0 {
				autoOverlap = math.Max(autoOverlap, math.Min(0, 35-previousDuration+autoPreutter))
			}
			// 語末子音を持つ境界では次onsetの食い込みを制限し、coda末尾を残す。
			if protectCoda && adjacent && len(previousUnit.CodaPhones) > 0 {
				autoPreutter, autoOverlap, codaLimited = CodaBoundaryOverlapMS(previousDuration, autoPreutter, autoOverlap)
			}
		}
		autoPreutter = math.Max(0, autoPreutter)
		result[index].Preutter = autoPreutter
		result[index].Overlap = autoOverlap
		result[index].Overlapped = previous >= 0 && adjacent && autoOverlap > 0
		result[index].CodaLimited = codaLimited
		if previous >= 0 {
			if adjacent {
				result[previous].TailIntrude = math.Max(result[previous].TailIntrude, math.Max(autoPreutter, autoPreutter-autoOverlap))
				result[previous].TailOverlap = math.Max(result[previous].TailOverlap, math.Max(autoOverlap, 0))
			}
		}
		if unit.Role != "transition" || cvvcTiming == CVVCTimingSequential {
			previous = index
		}
	}
	phraseStart := 0.0
	if first >= 0 {
		phraseStart = units[first].NoteStartMS - result[first].Preutter
	}
	return result, phraseStart
}

// OpenUtauEnvelopeFromTimingは音素時間からOpenUTAU互換の5点エンベロープを作る。
func OpenUtauEnvelopeFromTiming(unit plan.Unit, timing OpenUtauPhoneTiming) []WorldlineEnvelopePoint {
	fadeIn := 5.0
	if timing.Overlapped {
		fadeIn = timing.Overlap
	}
	fadeOut := 35.0
	if timing.TailOverlap > 0 {
		fadeOut = timing.TailOverlap
	}
	p0 := -timing.Preutter
	p1 := math.Max(p0+5, p0+fadeIn)
	p2 := math.Max(0, p1)
	p4 := unit.DurationMS - timing.TailIntrude + timing.TailOverlap
	p3 := math.Max(p2, p4-fadeOut)
	return []WorldlineEnvelopePoint{
		{XMS: p0, Y: 0}, {XMS: p1, Y: 1}, {XMS: p2, Y: 1},
		{XMS: p3, Y: 1}, {XMS: p4, Y: 0},
	}
}

// CVVCPreBoundaryEnvelopeはCVVC遷移音の境界フェードを手前へ寄せる。
func CVVCPreBoundaryEnvelope(points []WorldlineEnvelopePoint, timing OpenUtauPhoneTiming) []WorldlineEnvelopePoint {
	if len(points) != 5 {
		return points
	}
	result := append([]WorldlineEnvelopePoint(nil), points...)
	fadeOut := math.Max(5, timing.TailOverlap)
	fadeStart := math.Max(result[1].XMS, -fadeOut)
	result[2].XMS = fadeStart
	result[3].XMS = fadeStart
	result[4].XMS = 0
	return result
}

// ProviderPitchCurveはprovider用のピッチカーブへ複製する。
func ProviderPitchCurve(curve *PitchCurve) *provider.PitchCurve {
	if curve == nil {
		return nil
	}
	return &provider.PitchCurve{FrameMS: curve.FrameMS, Cents: append([]float64(nil), curve.Cents...)}
}
