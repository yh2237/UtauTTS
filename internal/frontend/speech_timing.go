package frontend

import (
	"math"
	"strings"
)

func englishSyllablePhones(s englishSyllable) []Phone {
	var phones []Phone
	for _, symbol := range s.onset {
		phones = append(phones, Phone{symbol, "onset"})
	}
	phones = append(phones, Phone{s.vowel, "nucleus"})
	for _, symbol := range s.coda {
		phones = append(phones, Phone{symbol, "coda"})
	}
	return phones
}

// 原音形式によらず、全レンダラーで同じ重みを使う。
func PhoneWeight(symbol, role string) float64 {
	if role == "nucleus" {
		return 1
	}
	switch strings.ToLower(symbol) {
	case "s", "sh", "f", "th", "z", "zh", "v", "dh", "ts":
		return 0.65
	case "ch", "jh":
		return 0.55
	case "p", "t", "k", "b", "d", "g", "dx":
		return 0.35
	case "m", "n", "ng", "l", "r":
		return 0.5
	default:
		return 0.45
	}
}

func PhoneSpans(phones []Phone, duration float64) []float64 {
	weights := make([]float64, len(phones))
	rhymeShares := MandarinRhymeShares(phones)
	for i, p := range phones {
		weights[i] = PhoneWeight(p.Symbol, p.Role)
		if rhymeShares[i] > 0 {
			weights[i] = rhymeShares[i]
		}
	}
	return PhoneSpansFromWeights(weights, duration)
}

// PhoneSpansFromWeightsは正の重みだけを合計して時間へ按分する。無効値は0として扱う。
func PhoneSpansFromWeights(weights []float64, duration float64) []float64 {
	spans := make([]float64, len(weights))
	total := 0.0
	for _, weight := range weights {
		if weight > 0 && !math.IsNaN(weight) && !math.IsInf(weight, 0) {
			total += weight
		}
	}
	if total <= 0 {
		return spans
	}
	for i, weight := range weights {
		spans[i] = duration * weight / total
	}
	return spans
}
