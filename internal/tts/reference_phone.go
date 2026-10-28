package tts

import (
	"fmt"
	"math"

	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/refspeech"
)

// phonePlanは、1モーラのなかの音素をどう配分するかの指定。
type phonePlan struct {
	// weightsは音素ごとの長さの比。nilなら指定がない。
	weights []float64
	// consonantMSとvowelMSは子音と母音の合計長（ミリ秒）。0なら指定がない。
	consonantMS float64
	vowelMS     float64
}

// buildPhonePlansは、設定された音素の指定をモーラごとの計画へまとめる。
// 配列は休止を含む全モーラと同じ長さで、休止は0にする。
func buildPhonePlans(cfg Config, morae []frontend.Mora) ([]phonePlan, error) {
	if len(cfg.ConsonantMS) == 0 && len(cfg.VowelMS) == 0 && len(cfg.PhoneWeights) == 0 {
		return nil, nil
	}
	if len(cfg.ConsonantMS) != 0 && (len(cfg.ConsonantMS) != len(morae) || len(cfg.VowelMS) != len(morae)) {
		return nil, fmt.Errorf("consonant_ms and vowel_ms must have %d entries including pauses", len(morae))
	}
	plans := make([]phonePlan, len(morae))
	for i, mora := range morae {
		if len(cfg.PhoneWeights) > i && len(cfg.PhoneWeights[i]) > 0 {
			if len(cfg.PhoneWeights[i]) != len(mora.Phones) {
				return nil, fmt.Errorf("phone weights at mora %d: got %d values for %d phones", i, len(cfg.PhoneWeights[i]), len(mora.Phones))
			}
			plans[i].weights = cfg.PhoneWeights[i]
		}
		if len(cfg.ConsonantMS) == 0 {
			continue
		}
		consonant, vowel := cfg.ConsonantMS[i], cfg.VowelMS[i]
		if consonant < 0 || vowel < 0 {
			return nil, fmt.Errorf("mora %d has a negative phone length", i)
		}
		if mora.Pause {
			if consonant != 0 || vowel != 0 {
				return nil, fmt.Errorf("mora %d is a pause; consonant_ms and vowel_ms must be 0", i)
			}
			continue
		}
		if consonant == 0 && vowel == 0 {
			continue
		}
		var consonants, nuclei int
		for _, phone := range mora.Phones {
			if phone.Role == "nucleus" {
				nuclei++
			} else {
				consonants++
			}
		}
		if consonants == 0 && vowel == 0 {
			return nil, fmt.Errorf("mora %d has no vowel phone", i)
		}
		if nuclei == 0 && consonant == 0 {
			return nil, fmt.Errorf("mora %d has no consonant phone", i)
		}
		if consonant > 0 && consonants == 0 {
			return nil, fmt.Errorf("mora %d has no consonant phone", i)
		}
		if vowel > 0 && nuclei == 0 {
			return nil, fmt.Errorf("mora %d has no vowel phone", i)
		}
		plans[i].consonantMS = consonant
		plans[i].vowelMS = vowel
	}
	return plans, nil
}

// splitTokensByPhoneは、モーラの記号を音素ごとに分ける。
// 記号数と音素数が違うときは、音素の数に比例して割り当てる。
func splitTokensByPhone(tokens []int, phones []frontend.Phone) [][]int {
	groups := make([][]int, len(phones))
	if len(phones) == 0 {
		return groups
	}
	at := 0
	for j := range phones {
		upto := int(math.Round(float64(len(tokens)*(j+1)) / float64(len(phones))))
		if upto > len(tokens) {
			upto = len(tokens)
		}
		if upto < at {
			upto = at
		}
		groups[j] = append(groups[j], tokens[at:upto]...)
		at = upto
	}
	if at < len(tokens) {
		last := len(groups) - 1
		groups[last] = append(groups[last], tokens[at:]...)
	}
	return groups
}

// phoneDurationsForTargetは、目標のモラ長を音素へ配分する。
// 子音は元の長さを保ち、余った時間を母音へ渡す。縮める場合は全体を比例配分する。
func phoneDurationsForTarget(phones []frontend.Phone, natural []float64, targetMS float64) []float64 {
	result := make([]float64, len(phones))
	var consonant, vowel, total float64
	for i, phone := range phones {
		total += natural[i]
		if phone.Role == "nucleus" {
			vowel += natural[i]
		} else {
			consonant += natural[i]
		}
	}
	if targetMS >= consonant && vowel > 0 {
		remainder := targetMS - consonant
		for i, phone := range phones {
			if phone.Role == "nucleus" {
				result[i] = natural[i] * remainder / vowel
			} else {
				result[i] = natural[i]
			}
		}
		return result
	}
	if total <= 0 {
		return result
	}
	scale := targetMS / total
	for i := range result {
		result[i] = natural[i] * scale
	}
	return result
}

// moraPhonePlanは、モーラの目標長と、その内訳の音素の長さ。
type moraPhonePlan struct {
	// phoneDurationsは音素ごとの目標長（ミリ秒）。nilなら指定がない。
	phoneDurations []float64
	// totalはモーラ全体の目標長（ミリ秒）。0なら指定がない。
	total float64
}

// referencePhonePlansは、モーラごとの目標長と音素の配分を求める。
// 目標が無いモーラはnilを返す。
func referencePhonePlans(morae []frontend.Mora, moraTokens [][]int, timing refspeech.Timing, manual []float64, plans []phonePlan) []moraPhonePlan {
	result := make([]moraPhonePlan, len(morae))
	for i, mora := range morae {
		if mora.Pause || len(mora.Phones) == 0 || i >= len(moraTokens) {
			continue
		}
		groups := splitTokensByPhone(moraTokens[i], mora.Phones)
		natural := make([]float64, len(mora.Phones))
		for j, group := range groups {
			for _, token := range group {
				natural[j] += timing.MS(token)
			}
		}
		var plan phonePlan
		if i < len(plans) {
			plan = plans[i]
		}
		if plan.consonantMS > 0 || plan.vowelMS > 0 {
			var consonants, nuclei int
			for _, phone := range mora.Phones {
				if phone.Role == "nucleus" {
					nuclei++
				} else {
					consonants++
				}
			}
			durations := make([]float64, len(mora.Phones))
			for j, phone := range mora.Phones {
				switch {
				case phone.Role == "nucleus" && nuclei > 0:
					durations[j] = plan.vowelMS / float64(nuclei)
				case phone.Role != "nucleus" && consonants > 0:
					durations[j] = plan.consonantMS / float64(consonants)
				default:
					durations[j] = 0
				}
			}
			result[i] = moraPhonePlan{phoneDurations: durations, total: plan.consonantMS + plan.vowelMS}
			continue
		}
		if len(plan.weights) == len(mora.Phones) {
			durations := frontend.PhoneSpansFromWeights(plan.weights, sumDurations(natural))
			result[i] = moraPhonePlan{phoneDurations: durations, total: sumDurations(durations)}
			continue
		}
		target, ok := configuredMoraDuration(i, manual)
		if !ok {
			continue
		}
		durations := phoneDurationsForTarget(mora.Phones, natural, target)
		result[i] = moraPhonePlan{phoneDurations: durations, total: target}
	}
	return result
}

func sumDurations(values []float64) float64 {
	var total float64
	for _, value := range values {
		total += value
	}
	return total
}

func configuredMoraDuration(position int, durations []float64) (float64, bool) {
	if position < 0 || position >= len(durations) {
		return 0, false
	}
	duration := durations[position]
	if duration <= 0 {
		return 0, false
	}
	return duration, true
}
