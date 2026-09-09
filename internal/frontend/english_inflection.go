package frontend

import "strings"

// Only use dictionary-backed stems. Exact entries are resolved by the caller
// first so irregular forms and lexical stress take precedence over these rules.
func englishInflectedPronunciation(word string) (string, error) {
	word = strings.ToLower(word)
	type candidate struct{ stem, ending string }
	var candidates []candidate
	switch {
	case strings.HasSuffix(word, "'s"):
		candidates = append(candidates, candidate{strings.TrimSuffix(word, "'s"), "s"})
	case strings.HasSuffix(word, "s'"):
		candidates = append(candidates, candidate{strings.TrimSuffix(word, "'"), ""})
	case strings.HasSuffix(word, "ies"):
		candidates = append(candidates, candidate{strings.TrimSuffix(word, "ies") + "y", "s"})
	case strings.HasSuffix(word, "s") && !strings.HasSuffix(word, "ss"):
		stem := strings.TrimSuffix(word, "s")
		candidates = append(candidates, candidate{stem, "s"})
		if strings.HasSuffix(word, "es") {
			stem = strings.TrimSuffix(word, "es")
			if strings.HasSuffix(stem, "s") || strings.HasSuffix(stem, "x") || strings.HasSuffix(stem, "z") || strings.HasSuffix(stem, "ch") || strings.HasSuffix(stem, "sh") {
				candidates = append(candidates, candidate{stem, "s"})
			}
		}
	case strings.HasSuffix(word, "ied"):
		candidates = append(candidates, candidate{strings.TrimSuffix(word, "ied") + "y", "ed"})
	case strings.HasSuffix(word, "ed") || strings.HasSuffix(word, "ing"):
		ending := "ed"
		if strings.HasSuffix(word, "ing") {
			ending = "ing"
		}
		stem := strings.TrimSuffix(word, ending)
		candidates = append(candidates, candidate{stem, ending}, candidate{stem + "e", ending})
		if len(stem) >= 3 && stem[len(stem)-1] == stem[len(stem)-2] && strings.ContainsRune("bdgmnprt", rune(stem[len(stem)-1])) {
			candidates = append(candidates, candidate{stem[:len(stem)-1], ending})
		}
	}
	result := ""
	for _, c := range candidates {
		if len(c.stem) < 2 {
			continue
		}
		reading, err := lookupEnglishDictionary(c.stem)
		if err != nil {
			return "", err
		}
		if reading == "" {
			continue
		}
		phones := strings.Fields(reading)
		last := strings.TrimRight(phones[len(phones)-1], "012")
		suffix := ""
		switch c.ending {
		case "s":
			switch last {
			case "S", "Z", "SH", "ZH", "CH", "JH":
				suffix = "IH0 Z"
			case "P", "T", "K", "F", "TH":
				suffix = "S"
			default:
				suffix = "Z"
			}
		case "ed":
			switch last {
			case "T", "D":
				suffix = "IH0 D"
			case "P", "K", "F", "TH", "S", "SH", "CH":
				suffix = "T"
			default:
				suffix = "D"
			}
		case "ing":
			suffix = "IH0 NG"
		}
		inferred := strings.TrimSpace(reading + " " + suffix)
		// CMUdict also contains names. A short spelling such as "mak" may
		// match alongside "make"; leave ambiguous recovery to the fallback.
		if result != "" && result != inferred {
			return "", nil
		}
		result = inferred
	}
	return result, nil
}
