package frontend

// CodaVowelCandidatesは語末子音の次に独立母音の候補を追加する。
func CodaVowelCandidates(morae []Mora, phonemizer string) []Mora {
	symbols := deltaEnglishSymbols
	if phonemizer == PhonemizerEnglishVCCV {
		symbols = vccvEnglishSymbols
	} else if phonemizer != PhonemizerEnglishDelta {
		return morae
	}
	result := append([]Mora(nil), morae...)
	for i := 1; i < len(morae); i++ {
		a, b := morae[i-1], morae[i]
		if a.Pause || b.Pause || a.WordIndex == b.WordIndex || b.Aliases == nil {
			continue
		}
		coda := false
		for _, p := range a.Phones {
			if p.Role == "coda" {
				coda = true
			}
		}
		onset := false
		s := englishSyllable{stress: b.Stress, stressKnown: b.StressKnown}
		for _, p := range b.Phones {
			if p.Role == "onset" {
				onset = true
			}
			if p.Role == "nucleus" {
				s.vowel = p.Symbol
			}
		}
		if !coda || onset || s.vowel == "" {
			continue
		}
		h := *b.Aliases
		h.Main = englishMainAliases(nil, englishSyllableVowels(s, symbols), symbols)
		h.MainKinds = repeatAliasKind("cv", len(h.Main))
		h.Transition = nil
		result[i].Aliases = &h
	}
	return result
}
