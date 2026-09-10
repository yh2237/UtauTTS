package frontend

import (
	"fmt"
	"strings"
)

// These are deliberately small, locally maintained spelling rules. They do not
// infer lexical stress: vowels without a stress digit remain StressKnown=false.
// Exact dictionary entries and unambiguous dictionary-backed inflections win.
func englishRulePronunciation(word string) (string, error) {
	word = strings.ToLower(strings.ReplaceAll(word, "’", "'"))
	if word == "" || strings.Trim(word, "abcdefghijklmnopqrstuvwxyz'") != "" || strings.Trim(word, "'") == "" {
		return "", fmt.Errorf("cannot infer English pronunciation for %q; specify an ARPAbet reading", word)
	}
	word = strings.ReplaceAll(word, "'", "")
	return strings.Join(englishSpellingPhones(word), " "), nil
}

type englishSpellingRule struct{ spelling, phones string }

// Longest groups precede their prefixes. All phonemes use the existing ARPAbet
// inventory, so this fallback works with ARPAsing, Delta and VCCV alike.
var englishSpellingGroups = []englishSpellingRule{
	{"eigh", "EY"}, {"igh", "AY"}, {"tion", "SH AH N"}, {"sion", "ZH AH N"},
	{"tch", "CH"}, {"dge", "JH"}, {"air", "EH R"},
	{"sh", "SH"}, {"ch", "CH"}, {"th", "TH"}, {"ph", "F"}, {"ng", "NG"},
	{"ck", "K"}, {"qu", "K W"}, {"wh", "W"},
	{"ee", "IY"}, {"ea", "IY"}, {"ai", "EY"}, {"ay", "EY"},
	{"oa", "OW"}, {"oo", "UW"}, {"ou", "AW"}, {"oi", "OY"}, {"oy", "OY"},
	{"au", "AO"}, {"aw", "AO"}, {"ie", "IY"}, {"ei", "EY"}, {"ew", "UW"},
	{"er", "ER"}, {"ir", "ER"}, {"ur", "ER"}, {"ar", "AA R"}, {"or", "AO R"},
}

func englishSpellingPhones(word string) []string {
	vowel := func(c byte) bool { return strings.ContainsRune("aeiouy", rune(c)) }
	var phones []string
	for i := 0; i < len(word); {
		rest := word[i:]
		if i == 0 && len(rest) > 1 {
			switch rest[:2] {
			case "kn", "gn", "pn", "wr", "ps":
				i++
				continue
			}
		}
		if rest == "mb" {
			phones = append(phones, "M")
			break
		}
		if rest == "le" && i > 0 && !vowel(word[i-1]) {
			phones = append(phones, "AH", "L")
			break
		}
		if rest == "e" && i > 1 && strings.ContainsAny(word[:i], "aeiouy") {
			break
		}
		if strings.HasPrefix(rest, "ow") {
			p := "AW"
			if len(rest) == 2 {
				p = "OW"
			}
			phones = append(phones, p)
			i += 2
			continue
		}
		matched := false
		for _, rule := range englishSpellingGroups {
			if strings.HasPrefix(rest, rule.spelling) {
				phones = append(phones, strings.Fields(rule.phones)...)
				i += len(rule.spelling)
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		c := word[i]
		p := ""
		switch c {
		case 'a', 'e', 'i', 'o', 'u':
			p = map[byte]string{'a': "AE", 'e': "EH", 'i': "IH", 'o': "AA", 'u': "AH"}[c]
			// A final silent e lengthens a preceding vowel across one consonant.
			if len(rest) == 3 && rest[2] == 'e' && !vowel(rest[1]) && !strings.ContainsRune("rvw", rune(rest[1])) {
				p = map[byte]string{'a': "EY", 'e': "IY", 'i': "AY", 'o': "OW", 'u': "UW"}[c]
			}
		case 'c':
			p = "K"
			if len(rest) > 1 && strings.ContainsRune("eiy", rune(rest[1])) {
				p = "S"
			}
		case 'g':
			p = "G"
			if len(rest) > 1 && strings.ContainsRune("eiy", rune(rest[1])) {
				p = "JH"
			}
		case 'x':
			p = "K S"
			if i == 0 {
				p = "Z"
			}
		case 'y':
			p = "Y"
			if i > 0 {
				p = "IH"
				if len(rest) == 1 {
					p = "IY"
					if !strings.ContainsAny(word[:i], "aeiou") {
						p = "AY"
					}
				}
			}
		case 's':
			p = "S"
			if i > 0 && len(rest) > 1 && vowel(word[i-1]) && vowel(rest[1]) {
				p = "Z"
			}
		default:
			p = map[byte]string{'b': "B", 'd': "D", 'f': "F", 'h': "HH", 'j': "JH", 'k': "K", 'l': "L", 'm': "M", 'n': "N", 'p': "P", 'q': "K", 'r': "R", 't': "T", 'v': "V", 'w': "W", 'z': "Z"}[c]
		}
		phones = append(phones, strings.Fields(p)...)
		i++
		if !vowel(c) && i < len(word) && word[i] == c {
			i++
		}
	}
	return phones
}

// A single productive prefix may be attached to a known stem or inflection.
// This is bounded (no recursive stripping) and does not split arbitrary words.
func englishPrefixedPronunciation(word string) (string, error) {
	word = strings.ToLower(word)
	for _, prefix := range []englishSpellingRule{
		{"micro", "M AY2 K R OW0"}, {"nano", "N AE2 N OW0"},
		{"hyper", "HH AY2 P ER0"}, {"cyber", "S AY2 B ER0"},
		{"non", "N AA2 N"}, {"pre", "P R IY0"},
		{"un", "AH0 N"}, {"re", "R IY0"},
	} {
		if !strings.HasPrefix(word, prefix.spelling) {
			continue
		}
		stem := strings.TrimPrefix(word, prefix.spelling)
		if len(stem) < 4 {
			continue
		}
		reading, err := lookupEnglishDictionary(stem)
		if err != nil {
			return "", err
		}
		if reading == "" {
			reading, err = englishInflectedPronunciation(stem)
		}
		if err != nil {
			return "", err
		}
		if reading != "" {
			return prefix.phones + " " + reading, nil
		}
	}
	return "", nil
}
