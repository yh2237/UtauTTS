package linguistics

import (
	"math"
	"strings"
)

// PhoneWeightは全レンダラーで共通の音素時間配分の重みを返す。
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

// MandarinRhymeSharesは介音・主母音・韻尾の構成に応じた時間配分比を返す。
func MandarinRhymeShares(phones []Phone) []float64 {
	result := make([]float64, len(phones))
	medial, offglide := false, false
	for _, p := range phones {
		medial = medial || p.Role == "medial"
		offglide = offglide || p.Role == "offglide"
	}
	nucleus := 1.0
	if medial {
		nucleus -= .2
	}
	if offglide {
		nucleus -= .3
	}
	for i, p := range phones {
		switch p.Role {
		case "medial":
			result[i] = .2
		case "offglide":
			result[i] = .3
		case "nucleus":
			result[i] = nucleus
		}
	}
	return result
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
