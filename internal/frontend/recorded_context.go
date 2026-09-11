package frontend

// RecordedContextCandidatesは語頭子音を残したままV-CV候補を追加する。
func RecordedContextCandidates(morae []Mora, phonemizer string) []Mora {
	if phonemizer != PhonemizerEnglishDelta && phonemizer != PhonemizerEnglishVCCV {
		return morae
	}
	symbols := deltaEnglishSymbols
	if phonemizer == PhonemizerEnglishVCCV {
		symbols = vccvEnglishSymbols
	}
	result := append([]Mora(nil), morae...)
	for i := 1; i < len(morae); i++ {
		previous, current := morae[i-1], morae[i]
		if previous.Pause || current.Pause || previous.Aliases == nil || current.Aliases == nil {
			continue
		}
		blocked := false
		for _, p := range previous.Phones {
			if p.Role == "coda" {
				blocked = true
			}
		}
		if blocked {
			continue
		}
		toSyllable := func(m Mora) englishSyllable {
			s := englishSyllable{stress: m.Stress, stressKnown: m.StressKnown}
			for _, p := range m.Phones {
				switch p.Role {
				case "onset":
					s.onset = append(s.onset, p.Symbol)
				case "nucleus":
					s.vowel = p.Symbol
				}
			}
			return s
		}
		a, b := toSyllable(previous), toSyllable(current)
		var contextual []string
		for _, v := range englishSyllableVowels(a, symbols) {
			for _, cv := range combineEnglishAliases(b.onset, englishSyllableVowels(b, symbols), symbols) {
				contextual = append(contextual, v+" "+cv)
			}
		}
		if len(contextual) == 0 {
			continue
		}
		hints := *current.Aliases
		hints.Main = append(contextual, current.Aliases.Main...)
		hints.MainKinds = append(repeatAliasKind("vcv", len(contextual)), current.Aliases.MainKinds...)
		result[i].Aliases = &hints
	}
	return result
}
