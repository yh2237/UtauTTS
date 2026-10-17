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
		rhymeShares := frontend.MandarinRhymeShares(mora.Phones)
		for j, phone := range mora.Phones {
			weight := frontend.PhoneWeight(phone.Symbol, phone.Role)
			if language == frontend.LanguageChinese {
				weight = mandarinPhoneWeight(mora, phone, weight)
				if rhymeShares[j] > 0 {
					weight = mandarinPhoneWeight(mora, frontend.Phone{Role: "nucleus"}, 1) * rhymeShares[j]
				}
			}
			weights[j] = weight
		}
		result[i] = weights
	}
	return result
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
