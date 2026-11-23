package tts

import "github.com/yh2237/UtauTTS/internal/linguistics"

func shouldUseLanguagePhoneTiming(language string, singleCV bool) bool {
	if language != linguistics.LanguageJapanese {
		return true
	}
	return singleCV
}

func languagePhoneWeights(language string, morae []linguistics.Mora) [][]float64 {
	result := make([][]float64, len(morae))
	for i, mora := range morae {
		if mora.Pause || len(mora.Phones) == 0 {
			continue
		}
		weights := make([]float64, len(mora.Phones))
		for j, phone := range mora.Phones {
			weights[j] = linguistics.PhoneWeight(phone.Symbol, phone.Role)
		}
		result[i] = weights
	}
	return result
}
