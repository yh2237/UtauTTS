package prosody

import (
	"fmt"
	"math"

	"utautts/internal/frontend"
)

func validateMoraPitchResidual(model *MoraPitchResidualModel, limits *ResidualLimits) error {
	if model == nil {
		return fmt.Errorf("missing mora_pitch_residual")
	}
	portable := &SequencePitchModel{
		FeatureNames: model.FeatureNames, InputWeights: model.InputWeights, InputBias: model.InputBias,
		Layers: model.Layers, OutputWeight: model.OutputWeight, OutputBias: model.OutputBias,
		Low: 0.01, High: 100,
	}
	if err := validateSequencePitch(portable); err != nil {
		return err
	}
	if limits == nil || limits.LowCents >= 0 || limits.HighCents <= 0 || limits.LowCents >= limits.HighCents || limits.SmoothingMS < 0 {
		return fmt.Errorf("invalid residual limits")
	}
	return nil
}

func (m *MoraPitchResidualModel) predict(morae []frontend.Mora, frames []FeatureFrame, base []float64, limits *ResidualLimits) ([]float64, bool) {
	if len(morae) == 0 || len(base) != len(morae) || validateMoraPitchResidual(m, limits) != nil {
		return nil, false
	}
	featureIndex := make(map[string]int, len(m.FeatureNames))
	for index, name := range m.FeatureNames {
		featureIndex[name] = index
	}
	baseFeatures := indexedFeatureVectors(morae, frames, featureIndex)
	phraseIDs := residualPhraseIDs(morae, frames)
	phraseMin, phraseMax := phraseBaseRanges(base, phraseIDs)
	hidden := len(m.InputBias)
	state := make([][]float64, len(morae))
	for position := range morae {
		state[position] = append([]float64(nil), m.InputBias...)
		for _, feature := range baseFeatures[position] {
			for output := 0; output < hidden; output++ {
				state[position][output] += m.InputWeights[output][feature.column] * feature.value
			}
		}
		previous := base[position]
		if position > 0 && phraseIDs[position] == phraseIDs[position-1] {
			previous = base[position-1]
		}
		next := base[position]
		if position+1 < len(base) && phraseIDs[position] == phraseIDs[position+1] {
			next = base[position+1]
		}
		addIndexedFeature(state[position], m.InputWeights, featureIndex, "base_pitch_cents", base[position]/120)
		addIndexedFeature(state[position], m.InputWeights, featureIndex, "base_prev_cents", previous/120)
		addIndexedFeature(state[position], m.InputWeights, featureIndex, "base_next_cents", next/120)
		addIndexedFeature(state[position], m.InputWeights, featureIndex, "base_delta_prev", (base[position]-previous)/120)
		addIndexedFeature(state[position], m.InputWeights, featureIndex, "base_delta_next", (next-base[position])/120)
		addIndexedFeature(state[position], m.InputWeights, featureIndex, "base_second_difference", (next-2*base[position]+previous)/120)
		if phraseIDs[position] >= 0 {
			addIndexedFeature(state[position], m.InputWeights, featureIndex, "base_phrase_min", phraseMin[position]/120)
			addIndexedFeature(state[position], m.InputWeights, featureIndex, "base_phrase_max", phraseMax[position]/120)
			addIndexedFeature(state[position], m.InputWeights, featureIndex, "base_phrase_range", (phraseMax[position]-phraseMin[position])/120)
		}
		addIndexedFeature(state[position], m.InputWeights, featureIndex, "base_near_render_limit", math.Abs(base[position])/90)
		for output := range state[position] {
			state[position][output] = math.Tanh(state[position][output])
		}
	}
	state = tcnForward(state, m.Layers, func(source, position int) bool {
		return phraseIDs[source] == phraseIDs[position]
	})
	result := make([]float64, len(morae))
	for position := range result {
		if morae[position].Pause {
			continue
		}
		value := m.OutputBias
		for index, weight := range m.OutputWeight {
			value += weight * state[position][index]
		}
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, false
		}
		result[position] = clamp(value, limits.LowCents, limits.HighCents)
	}
	return result, true
}

func (m *Model) applyMoraPitchResidual(base *PitchContour, morae []frontend.Mora, frames []FeatureFrame, timings []MoraTiming) *PitchContour {
	if base == nil || len(timings) != len(morae) || m.ResidualLimits == nil {
		return base
	}
	points := make([]float64, len(morae))
	for index, timing := range timings {
		points[index] = contourValueAt(base, timing.StartMS+timing.DurationMS/2)
	}
	residual, ok := m.MoraPitchResidual.predict(morae, frames, points, m.ResidualLimits)
	if !ok {
		return base
	}
	phraseIDs := residualPhraseIDs(morae, frames)
	centers := make([]float64, len(timings))
	for index, timing := range timings {
		centers[index] = timing.StartMS + timing.DurationMS/2
	}
	frameResidual := make([]float64, len(base.Cents))
	for frame := range frameResidual {
		timeMS := float64(frame) * base.FrameMS
		moraIndex := timingIndexAt(timings, timeMS)
		if moraIndex < 0 || phraseIDs[moraIndex] < 0 {
			continue
		}
		left, right := residualNeighbors(centers, phraseIDs, moraIndex, timeMS)
		if left == right || centers[right] <= centers[left] {
			frameResidual[frame] = residual[left]
		} else {
			ratio := clamp((timeMS-centers[left])/(centers[right]-centers[left]), 0, 1)
			frameResidual[frame] = residual[left]*(1-ratio) + residual[right]*ratio
		}
	}
	if m.ResidualLimits.SmoothingMS > 0 {
		frameResidual = smoothResidualFrames(frameResidual, base.FrameMS, m.ResidualLimits.SmoothingMS, timings, phraseIDs)
	}
	result := &PitchContour{FrameMS: base.FrameMS, Cents: append([]float64(nil), base.Cents...)}
	for index := range result.Cents {
		result.Cents[index] += frameResidual[index]
		result.Cents[index] = clamp(result.Cents[index], m.FramePitch.LowCents, m.FramePitch.HighCents)
	}
	return result
}

func contourValueAt(contour *PitchContour, timeMS float64) float64 {
	if contour == nil || len(contour.Cents) == 0 || contour.FrameMS <= 0 {
		return 0
	}
	position := clamp(timeMS/contour.FrameMS, 0, float64(len(contour.Cents)-1))
	left := int(math.Floor(position))
	right := min(left+1, len(contour.Cents)-1)
	ratio := position - float64(left)
	return contour.Cents[left]*(1-ratio) + contour.Cents[right]*ratio
}

func residualPhraseIDs(morae []frontend.Mora, frames []FeatureFrame) []int {
	result := make([]int, len(morae))
	phrase := -1
	for index, mora := range morae {
		if mora.Pause {
			result[index] = -1
			continue
		}
		start := index == 0 || morae[index-1].Pause
		if index < len(frames) && frames[index]["accent_phrase_start"] > 0 {
			start = true
		}
		if start {
			phrase++
		}
		result[index] = phrase
	}
	return result
}

func phraseBaseRanges(base []float64, phraseIDs []int) ([]float64, []float64) {
	minimums := make(map[int]float64)
	maximums := make(map[int]float64)
	for index, id := range phraseIDs {
		if id < 0 {
			continue
		}
		if _, ok := minimums[id]; !ok {
			minimums[id], maximums[id] = base[index], base[index]
		} else {
			minimums[id] = math.Min(minimums[id], base[index])
			maximums[id] = math.Max(maximums[id], base[index])
		}
	}
	mins, maxs := make([]float64, len(base)), make([]float64, len(base))
	for index, id := range phraseIDs {
		if id >= 0 {
			mins[index], maxs[index] = minimums[id], maximums[id]
		}
	}
	return mins, maxs
}

func timingIndexAt(timings []MoraTiming, timeMS float64) int {
	if len(timings) == 0 {
		return -1
	}
	for index := 0; index+1 < len(timings); index++ {
		if timeMS < timings[index].StartMS+timings[index].DurationMS {
			return index
		}
	}
	return len(timings) - 1
}

func residualNeighbors(centers []float64, phraseIDs []int, current int, timeMS float64) (int, int) {
	left, right := current, current
	for left > 0 && phraseIDs[left-1] == phraseIDs[current] && centers[left] > timeMS {
		left--
	}
	for right+1 < len(centers) && phraseIDs[right+1] == phraseIDs[current] && centers[right] < timeMS {
		right++
	}
	if centers[current] <= timeMS {
		left = current
		if current+1 < len(centers) && phraseIDs[current+1] == phraseIDs[current] {
			right = current + 1
		} else {
			right = current
		}
	} else {
		right = current
		if current > 0 && phraseIDs[current-1] == phraseIDs[current] {
			left = current - 1
		} else {
			left = current
		}
	}
	return left, right
}

func smoothResidualFrames(values []float64, frameMS, smoothingMS float64, timings []MoraTiming, phraseIDs []int) []float64 {
	radius := max(1, int(math.Ceil(smoothingMS/frameMS)))
	result := make([]float64, len(values))
	for index := range values {
		id := phraseIDs[timingIndexAt(timings, float64(index)*frameMS)]
		if id < 0 {
			continue
		}
		var sum, weights float64
		for offset := -radius; offset <= radius; offset++ {
			source := index + offset
			if source < 0 || source >= len(values) || phraseIDs[timingIndexAt(timings, float64(source)*frameMS)] != id {
				continue
			}
			weight := float64(radius + 1 - int(math.Abs(float64(offset))))
			sum += values[source] * weight
			weights += weight
		}
		if weights > 0 {
			result[index] = sum / weights
		}
	}
	return result
}
