package tts

import (
	"fmt"
	"math"

	"utautts/internal/frontend"
	"utautts/internal/openjtalk"
	"utautts/internal/prosody"
)

func alignRuntimeProsodyFeatures(morae []frontend.Mora, analysis *openjtalk.Analysis) ([]prosody.FeatureFrame, error) {
	if analysis == nil {
		return nil, fmt.Errorf("Open JTalk analysis is nil")
	}
	if len(analysis.Morae) != len(analysis.Features) {
		return nil, fmt.Errorf("Open JTalk returned %d morae and %d feature frames", len(analysis.Morae), len(analysis.Features))
	}
	if len(morae) == 0 {
		return nil, nil
	}
	if len(analysis.Morae) == 0 {
		return make([]prosody.FeatureFrame, len(morae)), nil
	}

	indices := alignRuntimeMoraIndices(morae, analysis.Morae)
	aligned := make([]prosody.FeatureFrame, len(morae))
	for index, analyzedIndex := range indices {
		if analyzedIndex >= 0 {
			aligned[index] = cloneFeatureFrame(analysis.Features[analyzedIndex])
			continue
		}
		if morae[index].Pause {
			aligned[index] = prosody.FeatureFrame{}
			continue
		}
		aligned[index] = cloneFeatureFrame(nearestRuntimeFeature(indices, analysis.Features, index))
	}
	return aligned, nil
}

type runtimeMoraAlignmentCell struct {
	cost float64
	op   byte
}

const (
	runtimeAlignmentSkipCost   = 1.1
	runtimeAlignmentChangeCost = 1.8
)

func alignRuntimeMoraIndices(morae []frontend.Mora, analyzed []string) []int {
	rows := len(morae) + 1
	columns := len(analyzed) + 1
	cells := make([][]runtimeMoraAlignmentCell, rows)
	for row := range cells {
		cells[row] = make([]runtimeMoraAlignmentCell, columns)
		for column := range cells[row] {
			cells[row][column].cost = math.Inf(1)
		}
	}
	cells[0][0] = runtimeMoraAlignmentCell{}
	for row := 1; row < rows; row++ {
		cells[row][0] = runtimeMoraAlignmentCell{
			cost: cells[row-1][0].cost + runtimeAlignmentSkipCost,
			op:   'g',
		}
	}
	for column := 1; column < columns; column++ {
		cells[0][column] = runtimeMoraAlignmentCell{
			cost: cells[0][column-1].cost + runtimeAlignmentSkipCost,
			op:   'o',
		}
	}
	for row := 1; row < rows; row++ {
		for column := 1; column < columns; column++ {
			best := runtimeMoraAlignmentCell{
				cost: cells[row-1][column-1].cost + runtimeMoraCost(morae[row-1], analyzed[column-1]),
				op:   'm',
			}
			best = chooseRuntimeAlignment(best, runtimeMoraAlignmentCell{
				cost: cells[row-1][column].cost + runtimeAlignmentSkipCost,
				op:   'g',
			})
			best = chooseRuntimeAlignment(best, runtimeMoraAlignmentCell{
				cost: cells[row][column-1].cost + runtimeAlignmentSkipCost,
				op:   'o',
			})
			cells[row][column] = best
		}
	}

	indices := make([]int, len(morae))
	for index := range indices {
		indices[index] = -1
	}
	row, column := len(morae), len(analyzed)
	for row > 0 || column > 0 {
		if row == 0 {
			column--
			continue
		}
		if column == 0 {
			row--
			continue
		}
		switch cells[row][column].op {
		case 'm':
			indices[row-1] = column - 1
			row--
			column--
		case 'g':
			row--
		case 'o':
			column--
		default:
			row--
			column--
		}
	}
	return indices
}

func chooseRuntimeAlignment(current, candidate runtimeMoraAlignmentCell) runtimeMoraAlignmentCell {
	if candidate.cost < current.cost-1e-9 {
		return candidate
	}
	if math.Abs(candidate.cost-current.cost) <= 1e-9 && runtimeAlignmentPriority(candidate.op) > runtimeAlignmentPriority(current.op) {
		return candidate
	}
	return current
}

func runtimeAlignmentPriority(operation byte) int {
	switch operation {
	case 'm':
		return 3
	case 'o':
		return 2
	case 'g':
		return 1
	default:
		return 0
	}
}

func runtimeMoraCost(mora frontend.Mora, analyzed string) float64 {
	if mora.Pause || analyzed == "" {
		if mora.Pause && analyzed == "" {
			return 0
		}
		return runtimeAlignmentChangeCost + runtimeAlignmentSkipCost
	}
	if mora.Text == analyzed {
		return 0
	}
	if analyzed == "ー" && isRuntimeVowel(mora.Vowel) {
		return 0.25
	}
	analyzedVowel := runtimeAnalyzedMoraVowel(analyzed)
	if analyzedVowel != "" && analyzedVowel == mora.Vowel {
		return 0.6
	}
	return runtimeAlignmentChangeCost
}

func isRuntimeVowel(vowel string) bool {
	switch vowel {
	case "a", "i", "u", "e", "o":
		return true
	default:
		return false
	}
}

func runtimeAnalyzedMoraVowel(analyzed string) string {
	parsed, err := frontend.ParseKana(analyzed)
	if err != nil || len(parsed) != 1 || parsed[0].Pause {
		return ""
	}
	return parsed[0].Vowel
}

func nearestRuntimeFeature(indices []int, features []prosody.FeatureFrame, target int) prosody.FeatureFrame {
	for distance := 1; distance < len(indices)+1; distance++ {
		left := target - distance
		if left >= 0 && indices[left] >= 0 {
			return features[indices[left]]
		}
		right := target + distance
		if right < len(indices) && indices[right] >= 0 {
			return features[indices[right]]
		}
	}
	return prosody.FeatureFrame{}
}

func cloneFeatureFrame(frame prosody.FeatureFrame) prosody.FeatureFrame {
	if len(frame) == 0 {
		return prosody.FeatureFrame{}
	}
	result := make(prosody.FeatureFrame, len(frame))
	for name, value := range frame {
		result[name] = value
	}
	return result
}
