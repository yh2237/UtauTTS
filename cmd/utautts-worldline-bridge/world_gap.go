package main

import "math"

const (
	worldGapMaxFrames       = 8
	worldGapBoundaryOverlap = 5.0
	worldGapAnchorBefore    = 120.0
	worldGapAnchorAfter     = 200.0
	worldGapEnergyRatio     = 0.10
	worldGapMinF0Frames     = 2
	worldGapMinEnergyFrames = 3
)

// 連続音の内部境界にある短い無声音区間だけを補間する。
func repairWorldFeatureGaps(input manifest, prepared []preparedWorldUnit, features *worldFeatures) int {
	if features == nil || len(features.F0) < 3 || len(input.Units) != len(prepared) {
		return 0
	}
	hasLegacy := false
	for _, item := range input.Units {
		if item.LegacyMix {
			hasLegacy = true
			break
		}
	}
	if !hasLegacy {
		return 0
	}
	repairedFrames := make([]bool, len(features.F0))
	repaired := 0
	for start := 0; start < len(features.F0); {
		if features.F0[start] > 71 {
			start++
			continue
		}
		end := start
		for end < len(features.F0) && features.F0[end] <= 71 {
			end++
		}
		if end-start >= worldGapMinF0Frames && end-start <= worldGapMaxFrames && start > 0 && end < len(features.F0) &&
			features.F0[start-1] > 71 && features.F0[end] > 71 &&
			worldGapBoundaryPair(input, prepared, start-1, end) &&
			worldGapInsideLegacyBoundary(input, prepared, start, end) {
			if interpolateWorldFeatureGap(features, start, end, start-1, end) {
				for frame := start; frame < end; frame++ {
					repairedFrames[frame] = true
				}
				repaired++
			}
		}
		start = end
	}
	energy := worldFeatureFrameEnergy(features)
	for index := 0; index+1 < len(input.Units); index++ {
		previous, next := input.Units[index], input.Units[index+1]
		if !worldLegacyBoundaryEligible(previous, next) {
			continue
		}
		left, right := worldBoundaryAnchors(input, prepared, features, energy, index, index+1)
		if left < 0 || right <= left+1 || right-left > int(math.Ceil((worldGapAnchorBefore+worldGapAnchorAfter)/worldFramePeriodMS))+4 {
			continue
		}
		scanStart, scanEnd := worldBoundaryScanRange(previous, next, left, right, len(features.F0))
		for start := scanStart; start < scanEnd; {
			if !worldBoundaryEnergyGap(energy, left, right, start) {
				start++
				continue
			}
			end := start + 1
			for end < scanEnd && worldBoundaryEnergyGap(energy, left, right, end) {
				end++
			}
			if end-start < worldGapMinEnergyFrames {
				start = end
				continue
			}
			if end-start > worldGapMaxFrames {
				center := start
				for frame := start + 1; frame < end; frame++ {
					if worldBoundaryEnergyRatio(energy, left, right, frame) < worldBoundaryEnergyRatio(energy, left, right, center) {
						center = frame
					}
				}
				start = center - worldGapMaxFrames/2
				if start < scanStart {
					start = scanStart
				}
				end = min(scanEnd, start+worldGapMaxFrames)
			}
			allRepaired := true
			for frame := start; frame < end; frame++ {
				if !repairedFrames[frame] {
					allRepaired = false
					break
				}
			}
			if !allRepaired && interpolateWorldFeatureGap(features, start, end, left, right) {
				for frame := start; frame < end; frame++ {
					repairedFrames[frame] = true
				}
				repaired++
			}
			start = end
		}
	}
	return repaired
}

func worldBoundaryScanRange(previous, next unit, left, right, frames int) (int, int) {
	if frames <= 0 {
		return 0, 0
	}
	previousEnd := previous.PositionMS + previous.LengthMS
	nextEnd := next.PositionMS + next.LengthMS
	overlapStart := math.Max(previous.PositionMS, next.PositionMS)
	overlapEnd := math.Min(previousEnd, nextEnd)
	start := max(left+1, int(math.Floor(overlapStart/worldFramePeriodMS)))
	end := min(right, int(math.Ceil(overlapEnd/worldFramePeriodMS)))
	start = min(max(0, start), frames)
	end = min(max(start, end), frames)
	return start, end
}

func worldGapBoundaryPair(input manifest, prepared []preparedWorldUnit, leftFrame, rightFrame int) bool {
	left := worldVoicedUnitAt(input, prepared, leftFrame)
	right := worldVoicedUnitAt(input, prepared, rightFrame)
	if left < 0 || right != left+1 || right >= len(input.Units) {
		return false
	}
	previous, next := input.Units[left], input.Units[right]
	if !worldLegacyBoundaryEligible(previous, next) {
		return false
	}
	return true
}

func worldGapInsideLegacyBoundary(input manifest, prepared []preparedWorldUnit, start, end int) bool {
	if start <= 0 || end <= start {
		return false
	}
	left := worldVoicedUnitAt(input, prepared, start-1)
	right := worldVoicedUnitAt(input, prepared, end)
	if left < 0 || right != left+1 || right >= len(input.Units) {
		return false
	}
	previous, next := input.Units[left], input.Units[right]
	if !worldLegacyBoundaryEligible(previous, next) {
		return false
	}
	overlapStart := math.Max(previous.PositionMS, next.PositionMS)
	overlapEnd := math.Min(previous.PositionMS+previous.LengthMS, next.PositionMS+next.LengthMS)
	return float64(start)*worldFramePeriodMS >= overlapStart && float64(end)*worldFramePeriodMS <= overlapEnd
}

func worldLegacyBoundaryEligible(previous, next unit) bool {
	if !previous.LegacyMix || !next.LegacyMix {
		return false
	}
	previousEnd := previous.PositionMS + previous.LengthMS
	nextEnd := next.PositionMS + next.LengthMS
	overlap := math.Min(previousEnd, nextEnd) - math.Max(previous.PositionMS, next.PositionMS)
	if overlap < worldGapBoundaryOverlap {
		return false
	}
	return true
}

func worldBoundaryAnchors(input manifest, prepared []preparedWorldUnit, features *worldFeatures, energy []float64, previous, next int) (int, int) {
	if previous < 0 || next >= len(input.Units) || len(energy) != len(features.F0) {
		return -1, -1
	}
	previousEnd := input.Units[previous].PositionMS + input.Units[previous].LengthMS
	leftStart := max(0, int(math.Floor((previousEnd-worldGapAnchorBefore)/worldFramePeriodMS)))
	leftEnd := min(len(features.F0)-1, int(math.Ceil((previousEnd+worldFramePeriodMS*2)/worldFramePeriodMS)))
	nextStart := max(0, int(math.Floor((input.Units[next].PositionMS-worldFramePeriodMS*2)/worldFramePeriodMS)))
	nextEnd := min(len(features.F0)-1, int(math.Ceil((input.Units[next].PositionMS+worldGapAnchorAfter)/worldFramePeriodMS)))
	left := -1
	for frame := leftStart; frame <= leftEnd; frame++ {
		if worldVoicedUnitAt(input, prepared, frame) != previous || energy[frame] <= 1e-9 {
			continue
		}
		if left < 0 || energy[frame] > energy[left] {
			left = frame
		}
	}
	right := -1
	for frame := nextStart; frame <= nextEnd; frame++ {
		if worldVoicedUnitAt(input, prepared, frame) != next || energy[frame] <= 1e-9 {
			continue
		}
		if right < 0 || energy[frame] > energy[right] {
			right = frame
		}
	}
	return left, right
}

func worldFeatureFrameEnergy(features *worldFeatures) []float64 {
	if features == nil || features.Frames <= 0 {
		return nil
	}
	bins := features.FFTSize/2 + 1
	energy := make([]float64, features.Frames)
	for frame := range energy {
		for bin := 0; bin < bins; bin++ {
			index := frame*bins + bin
			if index >= len(features.Spectrum) {
				break
			}
			value := features.Spectrum[index]
			if value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0) {
				energy[frame] += value
			}
		}
	}
	return energy
}

func worldBoundaryEnergyRatio(energy []float64, left, right, frame int) float64 {
	if left < 0 || right <= left || frame <= left || frame >= right || right >= len(energy) || energy[left] <= 1e-12 || energy[right] <= 1e-12 {
		return 1
	}
	fraction := float64(frame-left) / float64(right-left)
	expected := math.Exp(lerp(math.Log(energy[left]), math.Log(energy[right]), fraction))
	if expected <= 1e-12 || math.IsNaN(expected) || math.IsInf(expected, 0) {
		return 1
	}
	return energy[frame] / expected
}

func worldBoundaryEnergyGap(energy []float64, left, right, frame int) bool {
	if frame <= left || frame >= right || frame < 0 || frame >= len(energy) {
		return false
	}
	ratio := worldBoundaryEnergyRatio(energy, left, right, frame)
	return ratio < worldGapEnergyRatio && energy[frame] < math.Max(energy[left], energy[right])*.5
}

func worldVoicedUnitAt(input manifest, prepared []preparedWorldUnit, frame int) int {
	if frame < 0 || frame >= len(input.F0Curve) || len(input.Units) != len(prepared) {
		return -1
	}
	timeMS := float64(frame) * worldFramePeriodMS
	best, bestWeight := -1, 0.0
	for index, item := range input.Units {
		if !item.LegacyMix {
			continue
		}
		localMS := timeMS - item.PositionMS
		if localMS < 0 || localMS > item.LengthMS {
			continue
		}
		weight := worldEnvelopeWeight(item, localMS)
		if weight <= 1e-6 {
			continue
		}
		entry := prepared[index].cached
		if len(entry.features.F0) == 0 {
			continue
		}
		sourceMS := mapWorldFeatureTime(item, entry, localMS)
		sourceFrame := sourceMS / worldFramePeriodMS
		left := min(max(0, int(math.Floor(sourceFrame))), entry.features.Frames-1)
		right := min(left+1, entry.features.Frames-1)
		if left < 0 || right < 0 || left >= len(entry.features.F0) || right >= len(entry.features.F0) {
			continue
		}
		voiced := lerp(entry.features.F0[left], entry.features.F0[right], sourceFrame-float64(left))
		if voiced <= 71 {
			continue
		}
		weight *= worldUnitAmplitudeGain(item) * worldUnitAmplitudeGain(item)
		if weight > bestWeight {
			best, bestWeight = index, weight
		}
	}
	return best
}

func interpolateWorldFeatureGap(features *worldFeatures, start, end, left, right int) bool {
	if features == nil || start <= left || end > right || left < 0 || right >= len(features.F0) ||
		features.F0[left] <= 71 || features.F0[right] <= 71 || right <= left {
		return false
	}
	bins := features.FFTSize/2 + 1
	if bins <= 0 || len(features.Spectrum) < (right+1)*bins || len(features.Aperiodicity) < (right+1)*bins {
		return false
	}
	leftF0, rightF0 := features.F0[left], features.F0[right]
	for frame := start; frame < end; frame++ {
		fraction := float64(frame-left) / float64(right-left)
		features.F0[frame] = math.Exp(lerp(math.Log(leftF0), math.Log(rightF0), fraction))
		for bin := 0; bin < bins; bin++ {
			leftIndex, rightIndex := left*bins+bin, right*bins+bin
			valueLeft := math.Max(1e-12, features.Spectrum[leftIndex])
			valueRight := math.Max(1e-12, features.Spectrum[rightIndex])
			index := frame*bins + bin
			features.Spectrum[index] = math.Exp(lerp(math.Log(valueLeft), math.Log(valueRight), fraction))
			features.Aperiodicity[index] = math.Max(0, math.Min(1, lerp(features.Aperiodicity[leftIndex], features.Aperiodicity[rightIndex], fraction)))
		}
	}
	return true
}
