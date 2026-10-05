package frontend

import (
	"fmt"
	"strings"
)

func ParseChineseCVVC(text, reading string, dictionary map[string]string) (string, []Mora, error) {
	return ParseChineseCVVCWithConfig(text, reading, dictionary, PresampConfig{})
}

func ParseChineseCVVCWithConfig(text, reading string, dictionary map[string]string, config PresampConfig) (string, []Mora, error) {
	var syllables []string
	var tokens []chineseToken
	if strings.TrimSpace(reading) != "" {
		syllables = strings.Fields(reading)
	} else if value := dictionary[text]; value != "" {
		syllables = strings.Fields(value)
	} else {
		var err error
		tokens, err = chineseReadingTokens(text, dictionary)
		if err != nil {
			return "", nil, err
		}
		for _, token := range tokens {
			syllables = append(syllables, token.reading)
		}
	}
	var morae []Mora
	previousFinal := ""
	// プレビューの読みと一致する場合だけ再解析し、手動の読みは保つ。
	if len(tokens) == 0 && text != "" {
		if inferred, err := chineseReadingTokens(text, dictionary); err == nil {
			var values []string
			for _, token := range inferred {
				values = append(values, token.reading)
			}
			if strings.Join(values, " ") == strings.Join(syllables, " ") {
				tokens = inferred
			}
		}
	}
	phraseStart := true
	for syllableIndex, raw := range syllables {
		if raw == "|" {
			if len(morae) > 0 && !morae[len(morae)-1].Pause {
				setChineseEnding(&morae[len(morae)-1], config)
				morae = append(morae, Mora{Pause: true})
			}
			previousFinal, phraseStart = "", true
			continue
		}
		tone := pinyinTone(raw)
		syllable := normalizePinyin(raw)
		if replacement := config.Replacements[syllable]; replacement != "" {
			syllable = replacement
		}
		spellings := chineseAliasSpellings(syllable)
		initial, final := config.Consonants[syllable], config.Vowels[syllable]
		// WAV名とpresamp.iniでuとvが異なる音源にも対応する。
		for _, spelling := range spellings[1:] {
			if initial == "" {
				initial = config.Consonants[spelling]
			}
			if final == "" {
				final = config.Vowels[spelling]
			}
		}
		if final == "" {
			fallbackInitial, fallbackFinal := splitPinyin(syllable)
			if initial == "" {
				initial = fallbackInitial
			}
			final = fallbackFinal
		}
		if final == "" {
			return "", nil, fmt.Errorf("invalid Pinyin syllable %q", raw)
		}
		candidates := append([]string(nil), spellings...)
		kinds := repeatAliasKind("cv", len(spellings))
		if phraseStart {
			var starts []string
			for _, spelling := range spellings {
				starts = append(starts, "- "+spelling)
			}
			candidates = append(starts, candidates...)
			kinds = append(repeatAliasKind("vcv", len(starts)), kinds...)
		} else if previousFinal != "" {
			var connected []string
			for _, spelling := range spellings {
				connected = append(connected, previousFinal+" "+spelling)
			}
			candidates = append(connected, candidates...)
			kinds = append(repeatAliasKind("vcv", len(connected)), kinds...)
		}
		mora := Mora{Language: LanguageChinese, WordIndex: syllableIndex, WordEnd: true, Text: syllable, Consonant: initial, Vowel: final, Tone: tone, Aliases: &AliasHints{Main: candidates, MainKinds: kinds}}
		if len(tokens) == len(syllables) {
			mora.SourceText = tokens[syllableIndex].source
			mora.WordIndex = tokens[syllableIndex].word
			mora.WordEnd = tokens[syllableIndex].end
		}
		phoneInitial, phoneFinal := splitPinyin(normalizePinyin(raw))
		// 縮約表記の展開は発話計画だけに適用する。
		if expanded := map[string]string{"iu": "iou", "ui": "uei", "un": "uen"}[phoneFinal]; expanded != "" {
			phoneFinal = expanded
		}
		if phoneFinal == "uen" && (phoneInitial == "j" || phoneInitial == "q" || phoneInitial == "x" || phoneInitial == "y") {
			phoneFinal = "vn"
		}
		if phoneFinal == "ue" && (phoneInitial == "j" || phoneInitial == "q" || phoneInitial == "x" || phoneInitial == "y" || normalizePinyin(raw) == "nue" || normalizePinyin(raw) == "lue") {
			phoneFinal = "ve"
		}
		if phoneInitial != "" {
			mora.Phones = append(mora.Phones, Phone{phoneInitial, "onset"})
		}
		// alias用の韻母は保ち、目標時刻では鼻音韻尾を分離する。
		nucleus, coda := phoneFinal, ""
		if strings.HasSuffix(phoneFinal, "ng") {
			nucleus, coda = strings.TrimSuffix(phoneFinal, "ng"), "ng"
		} else if strings.HasSuffix(phoneFinal, "n") {
			nucleus, coda = strings.TrimSuffix(phoneFinal, "n"), "n"
		}
		if nucleus == "" {
			nucleus, coda = phoneFinal, ""
		}
		mora.Phones = append(mora.Phones, mandarinRhymeParts(nucleus)...)
		if coda != "" {
			mora.Phones = append(mora.Phones, Phone{coda, "coda"})
		}
		if previousFinal != "" && initial != "" {
			mora.Aliases.Transition = []string{previousFinal + " " + initial}
		}
		morae = append(morae, mora)
		previousFinal, phraseStart = final, false
	}
	if len(morae) == 0 {
		return "", nil, fmt.Errorf("Pinyin reading is empty")
	}
	for index := len(morae) - 1; index >= 0; index-- {
		if !morae[index].Pause {
			setChineseEnding(&morae[index], config)
			break
		}
	}
	return strings.Join(syllables, " "), morae, nil
}

func setChineseEnding(mora *Mora, config PresampConfig) {
	var endings []string
	for _, format := range config.Endings {
		endings = append(endings, strings.ReplaceAll(format, "%v%", mora.Vowel))
	}
	endings = append(endings, mora.Vowel+" R")
	mora.Aliases.Endings = [][]string{uniqueStrings(endings)}
}

func chineseAliasSpellings(syllable string) []string {
	// üeだけu表記とv表記を補完する。nuとnv、luとlvは別の母音として扱う。
	other := map[string]string{"nve": "nue", "nue": "nve", "lve": "lue", "lue": "lve"}[syllable]
	return uniqueStrings([]string{syllable, other})
}

func chineseSyllables(text string, dictionary map[string]string) ([]string, error) {
	tokens, err := chineseReadingTokens(text, dictionary)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, token := range tokens {
		result = append(result, token.reading)
	}
	return result, nil
}

var englishVowels = map[string]bool{
	"aa": true, "ae": true, "ah": true, "ao": true, "aw": true, "ax": true, "ay": true,
	"eh": true, "er": true, "ey": true, "ih": true, "iy": true, "ow": true,
	"oy": true, "uh": true, "uw": true,
}

var cvEnglishConsonants = map[string]bool{
	"b": true, "ch": true, "d": true, "dh": true, "dx": true, "f": true, "g": true,
	"hh": true, "jh": true, "k": true, "l": true, "m": true, "n": true, "ng": true,
	"p": true, "q": true, "r": true, "s": true, "sh": true, "t": true, "th": true,
	"v": true, "w": true, "y": true, "z": true, "zh": true,
}

var deltaEnglishSymbols = map[string][]string{
	"aa": {"A", "Q"}, "ae": {"{"}, "ah": {"V", "@"}, "ao": {"O", "Q"}, "ax": {"@", "V"},
	"aw": {"aU", "au"}, "ay": {"aI", "ai"}, "eh": {"E", "e"}, "er": {"3"},
	"ey": {"eI", "ei"}, "ih": {"I", "i"}, "iy": {"i"}, "ow": {"oU", "o"},
	"oy": {"OI", "oi"}, "uh": {"U"}, "uw": {"u"},
	"b": {"b"}, "ch": {"tS", "ch"}, "d": {"d"}, "dh": {"D", "dh"}, "dx": {"d", "dd"},
	"f": {"f"}, "g": {"g"}, "hh": {"h"}, "jh": {"dZ", "j"}, "k": {"k"},
	"l": {"l"}, "m": {"m"}, "n": {"n"}, "ng": {"N", "ng"}, "p": {"p"},
	"r": {"r"}, "s": {"s"}, "sh": {"S", "sh"}, "t": {"t"}, "th": {"T", "th"},
	"v": {"v"}, "w": {"w"}, "y": {"j", "y"}, "z": {"z"}, "zh": {"Z", "zh"},
}

var vccvEnglishSymbols = map[string][]string{
	"aa": {"a"}, "ae": {"@"}, "ah": {"u"}, "ao": {"9"}, "ax": {"x"},
	"aw": {"8"}, "ay": {"I"}, "eh": {"e"}, "er": {"3"},
	"ey": {"A"}, "ih": {"i"}, "iy": {"E"}, "ow": {"O"},
	"oy": {"Q"}, "uh": {"6"}, "uw": {"o"},
	"b": {"b"}, "ch": {"ch"}, "d": {"d", "dd"}, "dh": {"dh"}, "dx": {"dd", "d"}, "f": {"f"},
	"g": {"g"}, "hh": {"h"}, "jh": {"j"}, "k": {"k"}, "l": {"l"},
	"m": {"m"}, "n": {"n"}, "ng": {"ng"}, "p": {"p"}, "r": {"r"},
	"s": {"s"}, "sh": {"sh"}, "t": {"t"}, "th": {"th"}, "v": {"v"},
	"w": {"w"}, "y": {"y"}, "z": {"z"}, "zh": {"zh"},
}

func normalizePinyin(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimRight(value, "12345")
	value = strings.ReplaceAll(value, "u:", "v")
	value = strings.ReplaceAll(value, "ü", "v")
	return value
}

func pinyinTone(value string) int {
	value = strings.TrimSpace(value)
	if len(value) == 0 {
		return 0
	}
	last := value[len(value)-1]
	if last >= '1' && last <= '5' {
		return int(last - '0')
	}
	return 0
}

func splitPinyin(syllable string) (string, string) {
	initials := []string{"zh", "ch", "sh", "b", "p", "m", "f", "d", "t", "n", "l", "g", "k", "h", "j", "q", "x", "r", "z", "c", "s", "y", "w"}
	for _, initial := range initials {
		if strings.HasPrefix(syllable, initial) && len(syllable) > len(initial) {
			return initial, syllable[len(initial):]
		}
	}
	return "", syllable
}
