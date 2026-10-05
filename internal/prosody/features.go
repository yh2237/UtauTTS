package prosody

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"utautts/internal/frontend"
)

func smoothstep01(value float64) float64 {
	value = clamp(value, 0, 1)
	return value * value * (3 - 2*value)
}

func smoothFramePitchPhrases(values []float64, speech []bool, frameMS, sigmaMS float64) []float64 {
	result := append([]float64(nil), values...)
	if frameMS <= 0 || sigmaMS <= 0 || len(values) != len(speech) {
		return result
	}
	sigma := sigmaMS / frameMS
	radius := max(1, int(math.Ceil(3*sigma)))
	weights := make([]float64, 2*radius+1)
	for offset := -radius; offset <= radius; offset++ {
		weights[offset+radius] = math.Exp(-0.5 * math.Pow(float64(offset)/sigma, 2))
	}
	for start := 0; start < len(values); {
		if !speech[start] {
			start++
			continue
		}
		end := start + 1
		for end < len(values) && speech[end] {
			end++
		}
		for position := start; position < end; position++ {
			sum, weightSum := 0.0, 0.0
			for offset := -radius; offset <= radius; offset++ {
				source := max(start, min(end-1, position+offset))
				weight := weights[offset+radius]
				sum += values[source] * weight
				weightSum += weight
			}
			result[position] = sum / weightSum
		}
		start = end
	}
	return result
}

func absolutePercentile(values []float64, mask []bool, quantile float64) float64 {
	selected := make([]float64, 0, len(values))
	for index, value := range values {
		if index < len(mask) && mask[index] {
			selected = append(selected, math.Abs(value))
		}
	}
	if len(selected) == 0 {
		return 0
	}
	sort.Float64s(selected)
	index := int(math.Ceil(clamp(quantile, 0, 1)*float64(len(selected)))) - 1
	return selected[max(0, min(len(selected)-1, index))]
}

func featuresFor(morae []frontend.Mora, position int) map[string]float64 {
	english := morae[position].Language == frontend.LanguageEnglish
	if morae[position].Pause {
		english = english || (position > 0 && morae[position-1].Language == frontend.LanguageEnglish) || (position+1 < len(morae) && morae[position+1].Language == frontend.LanguageEnglish)
	}
	if english {
		return englishFrameFeatures(morae, position)
	}
	current := morae[position]
	denominator := float64(max(1, len(morae)-1))
	pos := float64(position) / denominator
	result := map[string]float64{
		"bias": 1, "position": pos, "position2": pos * pos,
		"from_end": 1 - pos,
	}
	if position == 0 || morae[position-1].Pause {
		result["phrase_start"] = 1
	}
	if position == len(morae)-1 || morae[position+1].Pause {
		result["phrase_end"] = 1
	}
	addCategorical(result, "mora", current)
	if position > 0 {
		addCategorical(result, "prev", morae[position-1])
	} else {
		result["prev=<BOS>"] = 1
	}
	if position+1 < len(morae) {
		addCategorical(result, "next", morae[position+1])
	} else {
		result["next=<EOS>"] = 1
	}
	return result
}

// 英語の特徴は録音aliasではなく音素・強勢から作る。
func englishFrameFeatures(morae []frontend.Mora, position int) map[string]float64 {
	current := morae[position]
	pos := float64(position) / float64(max(1, len(morae)-1))
	f := map[string]float64{"bias": 1, "position": pos, "position2": pos * pos, "from_end": 1 - pos}
	add := func(prefix string, unit frontend.Mora) {
		if unit.Pause {
			f[prefix+"=<PAUSE>"] = 1
			return
		}
		var symbols []string
		for _, phone := range unit.Phones {
			symbols = append(symbols, phone.Symbol)
			f[prefix+"_"+phone.Role+"="+phone.Symbol] = 1
		}
		f[prefix+"="+strings.Join(symbols, " ")] = 1
		if unit.StressKnown {
			f[fmt.Sprintf("%s_stress=%d", prefix, unit.Stress)] = 1
		}
	}
	add("syllable", current)
	if position == 0 {
		f["prev=<BOS>"] = 1
	} else {
		add("prev", morae[position-1])
	}
	if position+1 == len(morae) {
		f["next=<EOS>"] = 1
	} else {
		add("next", morae[position+1])
	}
	if position == 0 || morae[position-1].Pause {
		f["phrase_start"] = 1
	}
	if position+1 == len(morae) || morae[position+1].Pause {
		f["phrase_end"] = 1
	}
	if !current.Pause {
		if position == 0 || morae[position-1].Pause || morae[position-1].WordIndex != current.WordIndex {
			f["en_word_start"] = 1
		}
		if current.WordEnd {
			f["en_word_end"] = 1
		}
	}
	return f
}

// 静的特徴を使い回し、フレームごとのmap生成を避ける。
type indexedFeature struct {
	column int
	value  float64
}

func indexedFeatureVectors(morae []frontend.Mora, frames []FeatureFrame, index map[string]int) [][]indexedFeature {
	result := make([][]indexedFeature, len(morae))
	for position := range morae {
		features := featuresFor(morae, position)
		if position < len(frames) {
			for name, value := range frames[position] {
				features[name] = value
			}
		}
		row := make([]indexedFeature, 0, len(features))
		for name, value := range features {
			if column, ok := index[name]; ok {
				if value != 0 {
					row = append(row, indexedFeature{column: column, value: value})
				}
			}
		}
		result[position] = row
	}
	return result
}

func addIndexedFeature(state []float64, weights [][]float64, index map[string]int, name string, value float64) {
	column, ok := index[name]
	if !ok || value == 0 {
		return
	}
	for output := range state {
		state[output] += weights[output][column] * value
	}
}

func addCategorical(features map[string]float64, prefix string, mora frontend.Mora) {
	if mora.Pause {
		features[prefix+"=<PAUSE>"] = 1
		return
	}
	features[prefix+"="+mora.Text] = 1
	features[prefix+"_vowel="+mora.Vowel] = 1
}

func dot(weights, features map[string]float64) float64 {
	result := 0.0
	for _, name := range sortedFeatureNames(features) {
		result += weights[name] * features[name]
	}
	return result
}

func sortedFeatureNames(features map[string]float64) []string {
	names := make([]string, 0, len(features))
	for name := range features {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func clamp(value, low, high float64) float64 {
	return math.Max(low, math.Min(high, value))
}
