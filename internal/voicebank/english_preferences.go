package voicebank

import (
	"strings"

	"utautts/internal/frontend"
)

// 英語では子音文脈を持つCVVC候補を優先する
func applyEnglishCandidatePreferences(candidates []Selection, previousLayer []Selection) {
	if len(candidates) == 0 || candidates[0].Mora.Language != frontend.LanguageEnglish {
		return
	}
	crossWord := false
	if len(previousLayer) > 0 {
		previous := previousLayer[0].Mora
		current := candidates[0].Mora
		crossWord = !previous.Pause && !current.Pause && previous.WordIndex != current.WordIndex
	}
	for index := range candidates {
		candidate := &candidates[index]
		if candidate.Composite {
			candidate.PreferenceScore += 4
			if crossWord {
				candidate.PreferenceScore += 4
			}
		}
		if englishStopOnset(candidate.Mora) && englishOnsetPresent(candidate.Mora, candidate.Alias) {
			candidate.PreferenceScore += 3
		} else if englishStopOnset(candidate.Mora) && candidate.Kind != AliasVC {
			candidate.PreferenceScore -= 3
		}
	}
}

func englishEndingReleasePreference(selection Selection) float64 {
	if selection.Mora.Language != frontend.LanguageEnglish || !englishStopCoda(selection.Mora) {
		return 0
	}
	alias := strings.ReplaceAll(strings.TrimSpace(selection.Alias), " ", "")
	// 音階接尾辞の後ろに解放記号が残る音源にも対応する
	if strings.Contains(alias, "-") {
		return 6
	}
	return 0
}

func englishStopOnset(mora frontend.Mora) bool {
	for _, phone := range mora.Phones {
		if phone.Role == "onset" && isEnglishStop(phone.Symbol) {
			return true
		}
	}
	return false
}

func englishStopCoda(mora frontend.Mora) bool {
	for _, phone := range mora.Phones {
		if phone.Role == "coda" && isEnglishStop(phone.Symbol) {
			return true
		}
	}
	return false
}

func isEnglishStop(symbol string) bool {
	switch strings.ToLower(strings.TrimSpace(symbol)) {
	case "p", "b", "t", "d", "k", "g", "ch", "jh":
		return true
	default:
		return false
	}
}

func englishOnsetPresent(mora frontend.Mora, alias string) bool {
	var onset []string
	for _, phone := range mora.Phones {
		if phone.Role == "onset" {
			onset = append(onset, strings.ToLower(strings.TrimSpace(phone.Symbol)))
		}
	}
	if len(onset) == 0 {
		return true
	}
	compact := strings.ReplaceAll(strings.TrimLeft(strings.TrimSpace(alias), "-"), " ", "")
	prefixes := []string{""}
	for _, phone := range onset {
		alternatives := englishPhoneAliasForms(phone)
		next := make([]string, 0, len(prefixes)*len(alternatives))
		for _, prefix := range prefixes {
			for _, alternative := range alternatives {
				next = append(next, prefix+alternative)
			}
		}
		prefixes = next
	}
	for _, prefix := range prefixes {
		if prefix != "" && strings.Contains(compact, prefix) {
			return true
		}
	}
	return false
}

func englishPhoneAliasForms(phone string) []string {
	switch phone {
	case "ch":
		return []string{"ch", "tS"}
	case "jh":
		return []string{"jh", "dZ", "j"}
	case "dh":
		return []string{"dh", "D"}
	case "ng":
		return []string{"ng", "N"}
	case "sh":
		return []string{"sh", "S"}
	case "th":
		return []string{"th", "T"}
	case "zh":
		return []string{"zh", "Z"}
	default:
		return []string{phone}
	}
}
