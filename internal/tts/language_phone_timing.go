package tts

import "github.com/yh2237/UtauTTS/internal/frontend"

func shouldUseLanguagePhoneTiming(language string, singleCV bool) bool {
	if language != frontend.LanguageJapanese {
		return true
	}
	return singleCV
}

func languagePhoneWeights(language string, morae []frontend.Mora) [][]float64 {
	result := make([][]float64, len(morae))
	for i, mora := range morae {
		if mora.Pause || len(mora.Phones) == 0 {
			continue
		}
		weights := make([]float64, len(mora.Phones))
		for j, phone := range mora.Phones {
			weights[j] = frontend.PhoneWeight(phone.Symbol, phone.Role)
		}
		result[i] = weights
	}
	return result
}
