package tts

import (
	"math"
	"strings"

	"utautts/internal/frontend"
)

func languagePhoneWeights(language string, morae []frontend.Mora) [][]float64 {
	result := make([][]float64, len(morae))
	for i, mora := range morae {
		if mora.Pause || len(mora.Phones) == 0 {
			continue
		}
		weights := make([]float64, len(mora.Phones))
		for j, phone := range mora.Phones {
			weight := frontend.PhoneWeight(phone.Symbol, phone.Role)
			switch language {
			case frontend.LanguageEnglish:
				weight = englishPhoneWeight(mora, phone, weight)
			case frontend.LanguageChinese:
				weight = mandarinPhoneWeight(mora, phone, weight)
			}
			weights[j] = weight
		}
		result[i] = weights
	}
	return result
}

func phoneSpansFromWeights(weights []float64, duration float64) []float64 {
	result := make([]float64, len(weights))
	total := 0.0
	for _, weight := range weights {
		if weight > 0 && !math.IsNaN(weight) && !math.IsInf(weight, 0) {
			total += weight
		}
	}
	if total <= 0 {
		return result
	}
	for i, weight := range weights {
		result[i] = duration * weight / total
	}
	return result
}

func englishPhoneWeight(mora frontend.Mora, phone frontend.Phone, weight float64) float64 {
	if phone.Role == "nucleus" {
		switch mora.Stress {
		case 1:
			return weight * 1.12
		case 2:
			return weight * 1.06
		case 0:
			if mora.StressKnown {
				return weight * 0.9
			}
		}
	}
	if phone.Role != "coda" {
		return weight
	}
	switch strings.ToLower(phone.Symbol) {
	case "p", "t", "k", "b", "d", "g", "ch", "jh":
		return weight * 1.65
	case "s", "sh", "f", "th", "z", "zh", "v", "dh":
		return weight * 1.25
	default:
		return weight * 1.1
	}
}

func mandarinPhoneWeight(mora frontend.Mora, phone frontend.Phone, weight float64) float64 {
	if phone.Role != "nucleus" {
		return weight
	}
	switch mora.Tone {
	case 3:
		return weight * 1.12
	case 4:
		return weight * 0.94
	case 5:
		return weight * 0.78
	default:
		return weight
	}
}
