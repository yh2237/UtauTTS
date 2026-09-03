package frontend

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mozillazg/go-pinyin"
)

const (
	LanguageJapanese = "ja"
	LanguageEnglish  = "en"
	LanguageChinese  = "zh"

	PhonemizerJapanese     = "ja-kana"
	PhonemizerEnglish      = "en-arpasing"
	PhonemizerEnglishDelta = "en-delta"
	PhonemizerEnglishVCCV  = "en-vccv"
	PhonemizerChinese      = "zh-cvvc"
)

func ResolveLanguage(language, phonemizer string) (string, string, error) {
	language = strings.ToLower(strings.TrimSpace(language))
	phonemizer = strings.ToLower(strings.TrimSpace(phonemizer))
	if language == "" {
		switch phonemizer {
		case PhonemizerEnglish, PhonemizerEnglishDelta, PhonemizerEnglishVCCV:
			language = LanguageEnglish
		case PhonemizerChinese:
			language = LanguageChinese
		default:
			language = LanguageJapanese
		}
	}
	if phonemizer == "" {
		phonemizer = map[string]string{
			LanguageJapanese: PhonemizerJapanese,
			LanguageEnglish:  PhonemizerEnglish,
			LanguageChinese:  PhonemizerChinese,
		}[language]
	}
	valid := map[string]map[string]bool{
		LanguageJapanese: {PhonemizerJapanese: true},
		LanguageEnglish:  {PhonemizerEnglish: true, PhonemizerEnglishDelta: true, PhonemizerEnglishVCCV: true},
		LanguageChinese:  {PhonemizerChinese: true},
	}[language]
	if valid == nil {
		return "", "", fmt.Errorf("unsupported language %q", language)
	}
	if !valid[phonemizer] {
		return "", "", fmt.Errorf("phonemizer %q does not support language %q", phonemizer, language)
	}
	return language, phonemizer, nil
}

// ParseEnglishDeltaはARPAbet読みをデルタ式CVVCへ変換する。
func ParseEnglishDelta(text, reading string, dictionary map[string]string) (string, []Mora, error) {
	return parseEnglishSyllables(text, reading, dictionary, deltaEnglishSymbols, " ", true)
}

// ParseEnglishVCCVはARPAbet読みをCz式VCCVへ変換する。
func ParseEnglishVCCV(text, reading string, dictionary map[string]string) (string, []Mora, error) {
	return parseEnglishSyllables(text, reading, dictionary, vccvEnglishSymbols, "", false)
}

func parseEnglishSyllables(text, reading string, dictionary map[string]string, symbols map[string][]string, separator string, spacedStart bool) (string, []Mora, error) {
	pronunciation, arpabet, err := englishPronunciation(text, reading, dictionary)
	if err != nil {
		return "", nil, err
	}
	type syllable struct {
		onset []string
		vowel string
	}
	var syllables []syllable
	var onset []string
	for _, phoneme := range arpabet {
		mapped := symbols[phoneme]
		if len(mapped) == 0 {
			return "", nil, fmt.Errorf("unsupported ARPAbet phoneme %q", phoneme)
		}
		if englishVowels[phoneme] {
			syllables = append(syllables, syllable{onset: append([]string(nil), onset...), vowel: phoneme})
			onset = nil
		} else {
			onset = append(onset, phoneme)
		}
	}
	if len(syllables) == 0 {
		return "", nil, fmt.Errorf("ARPAbet reading contains no vowel")
	}
	units := make([]Mora, 0, len(syllables))
	previousVowels := []string(nil)
	for i, syllable := range syllables {
		main := combineEnglishAliases(syllable.onset, symbols[syllable.vowel], symbols)
		if i == 0 {
			prefix := "-"
			if spacedStart {
				prefix = "- "
			}
			for _, alias := range append([]string(nil), main...) {
				main = append(main, prefix+alias)
			}
			main = append(main[len(main)/2:], main[:len(main)/2]...)
		}
		transitions := combineEnglishTransitionAliases(previousVowels, syllable.onset, symbols, separator)
		units = append(units, Mora{Text: main[0], Consonant: strings.Join(syllable.onset, " "), Vowel: symbols[syllable.vowel][0], Aliases: &AliasHints{Main: uniqueStrings(main), Transition: uniqueStrings(transitions)}})
		previousVowels = symbols[syllable.vowel]
	}
	if len(onset) > 0 {
		endings := combineEnglishTransitionAliases(previousVowels, onset, symbols, separator)
		cluster := combineEnglishAliases(onset, []string{""}, symbols)
		for _, value := range cluster {
			endings = append(endings, value+separator+"-", value+"-")
		}
		units = append(units, Mora{Text: endings[0], Consonant: strings.Join(onset, " "), Aliases: &AliasHints{Main: uniqueStrings(endings)}})
	}
	return pronunciation, units, nil
}

func englishPronunciation(text, reading string, dictionary map[string]string) (string, []string, error) {
	pronunciation := strings.TrimSpace(reading)
	if pronunciation == "" {
		words := latinWords(text)
		if len(words) == 0 {
			return "", nil, fmt.Errorf("English text contains no words")
		}
		parts := make([]string, 0, len(words))
		for _, word := range words {
			value := dictionary[word]
			if value == "" {
				value = dictionary[strings.ToLower(word)]
			}
			if value == "" {
				var err error
				value, err = englishWordPronunciation(word)
				if err != nil {
					return "", nil, err
				}
			}
			parts = append(parts, value)
		}
		pronunciation = strings.Join(parts, " ")
	}
	fields := strings.Fields(pronunciation)
	if len(fields) == 0 {
		return "", nil, fmt.Errorf("ARPAbet reading is empty")
	}
	for i := range fields {
		fields[i] = normalizeARPAbet(fields[i])
	}
	return pronunciation, fields, nil
}

func combineEnglishAliases(onset []string, vowels []string, symbols map[string][]string) []string {
	aliases := []string{""}
	for _, phoneme := range append(append([]string(nil), onset...), "") {
		values := vowels
		if phoneme != "" {
			values = symbols[phoneme]
		}
		next := make([]string, 0, len(aliases)*len(values))
		for _, left := range aliases {
			for _, right := range values {
				next = append(next, left+right)
			}
		}
		aliases = next
	}
	return aliases
}

func combineEnglishTransitionAliases(previous []string, onset []string, symbols map[string][]string, separator string) []string {
	if len(previous) == 0 || len(onset) == 0 {
		return nil
	}
	cluster := combineEnglishAliases(onset, []string{""}, symbols)
	var result []string
	for _, vowel := range previous {
		for _, consonant := range cluster {
			result = append(result, vowel+separator+consonant)
		}
	}
	return result
}

func ParseEnglishARPAsing(text, reading string, dictionary map[string]string) (string, []Mora, error) {
	pronunciation := strings.TrimSpace(reading)
	if pronunciation == "" {
		words := latinWords(text)
		if len(words) == 0 {
			return "", nil, fmt.Errorf("English text contains no words")
		}
		parts := make([]string, 0, len(words))
		for _, word := range words {
			value := dictionary[word]
			if value == "" {
				value = dictionary[strings.ToLower(word)]
			}
			if value == "" {
				var err error
				value, err = englishWordPronunciation(word)
				if err != nil {
					return "", nil, err
				}
			}
			parts = append(parts, value)
		}
		pronunciation = strings.Join(parts, " ")
	}
	symbols := strings.Fields(pronunciation)
	if len(symbols) == 0 {
		return "", nil, fmt.Errorf("ARPAbet reading is empty")
	}
	morae := make([]Mora, 0, len(symbols))
	previous := "-"
	for _, raw := range symbols {
		symbol := normalizeARPAbet(raw)
		if symbol == "" {
			continue
		}
		candidates := []string{previous + " " + symbol, symbol}
		if previous != "-" {
			candidates = append(candidates, "- "+symbol)
		}
		mora := Mora{Text: symbol, Aliases: &AliasHints{Main: uniqueStrings(candidates)}}
		if englishVowels[symbol] {
			mora.Vowel = symbol
		} else {
			mora.Consonant = symbol
		}
		morae = append(morae, mora)
		previous = symbol
	}
	return strings.Join(symbols, " "), morae, nil
}

func ParseChineseCVVC(text, reading string, dictionary map[string]string) (string, []Mora, error) {
	var syllables []string
	if strings.TrimSpace(reading) != "" {
		syllables = strings.Fields(reading)
	} else if value := dictionary[text]; value != "" {
		syllables = strings.Fields(value)
	} else {
		var err error
		syllables, err = chineseSyllables(text, dictionary)
		if err != nil {
			return "", nil, err
		}
	}
	var morae []Mora
	previousFinal := ""
	phraseStart := true
	for _, raw := range syllables {
		if raw == "|" {
			if len(morae) > 0 && !morae[len(morae)-1].Pause {
				morae = append(morae, Mora{Pause: true})
			}
			previousFinal, phraseStart = "", true
			continue
		}
		syllable := normalizePinyin(raw)
		initial, final := splitPinyin(syllable)
		if final == "" {
			return "", nil, fmt.Errorf("invalid Pinyin syllable %q", raw)
		}
		candidates := []string{syllable}
		if phraseStart {
			candidates = append([]string{"- " + syllable}, candidates...)
		} else if previousFinal != "" {
			candidates = append([]string{previousFinal + " " + syllable}, candidates...)
		}
		mora := Mora{Text: syllable, Consonant: initial, Vowel: final, Aliases: &AliasHints{Main: candidates}}
		if previousFinal != "" && initial != "" {
			mora.Aliases.Transition = []string{previousFinal + " " + initial}
		}
		morae = append(morae, mora)
		previousFinal, phraseStart = final, false
	}
	if len(morae) == 0 {
		return "", nil, fmt.Errorf("Pinyin reading is empty")
	}
	return strings.Join(syllables, " "), morae, nil
}

func chineseSyllables(text string, dictionary map[string]string) ([]string, error) {
	args := pinyin.NewArgs()
	args.Style = pinyin.Normal
	keys := make([]string, 0, len(dictionary))
	for key := range dictionary {
		if key != "" {
			keys = append(keys, key)
		}
	}
	sort.SliceStable(keys, func(i, j int) bool { return len([]rune(keys[i])) > len([]rune(keys[j])) })
	var result []string
	for len(text) > 0 {
		matched := false
		for _, key := range keys {
			if strings.HasPrefix(text, key) {
				result = append(result, strings.Fields(dictionary[key])...)
				text = strings.TrimPrefix(text, key)
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		r, size := utf8.DecodeRuneInString(text)
		text = text[size:]
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			result = append(result, "|")
			continue
		}
		values := pinyin.SinglePinyin(r, args)
		if len(values) == 0 {
			return nil, fmt.Errorf("cannot convert %q to Pinyin; specify --reading", string(r))
		}
		result = append(result, values[0])
	}
	return result, nil
}

var englishVowels = map[string]bool{
	"aa": true, "ae": true, "ah": true, "ao": true, "aw": true, "ay": true,
	"eh": true, "er": true, "ey": true, "ih": true, "iy": true, "ow": true,
	"oy": true, "uh": true, "uw": true,
}

var deltaEnglishSymbols = map[string][]string{
	"aa": {"A", "Q"}, "ae": {"{"}, "ah": {"V", "@"}, "ao": {"O", "Q"},
	"aw": {"aU", "au"}, "ay": {"aI", "ai"}, "eh": {"E", "e"}, "er": {"3"},
	"ey": {"eI", "ei"}, "ih": {"I", "i"}, "iy": {"i"}, "ow": {"oU", "o"},
	"oy": {"OI", "oi"}, "uh": {"U"}, "uw": {"u"},
	"b": {"b"}, "ch": {"tS", "ch"}, "d": {"d"}, "dh": {"D", "dh"},
	"f": {"f"}, "g": {"g"}, "hh": {"h"}, "jh": {"dZ", "j"}, "k": {"k"},
	"l": {"l"}, "m": {"m"}, "n": {"n"}, "ng": {"N", "ng"}, "p": {"p"},
	"r": {"r"}, "s": {"s"}, "sh": {"S", "sh"}, "t": {"t"}, "th": {"T", "th"},
	"v": {"v"}, "w": {"w"}, "y": {"j", "y"}, "z": {"z"}, "zh": {"Z", "zh"},
}

var vccvEnglishSymbols = map[string][]string{
	"aa": {"Q", "A"}, "ae": {"A", "&"}, "ah": {"@", "6"}, "ao": {"0", "Q"},
	"aw": {"aW"}, "ay": {"aI"}, "eh": {"E", "e"}, "er": {"3", "0r"},
	"ey": {"A"}, "ih": {"I"}, "iy": {"i"}, "ow": {"0", "O"},
	"oy": {"OI"}, "uh": {"U"}, "uw": {"u"},
	"b": {"b"}, "ch": {"ch"}, "d": {"d", "dd"}, "dh": {"dh"}, "f": {"f"},
	"g": {"g"}, "hh": {"h"}, "jh": {"j"}, "k": {"k"}, "l": {"l"},
	"m": {"m"}, "n": {"n"}, "ng": {"ng"}, "p": {"p"}, "r": {"r"},
	"s": {"s"}, "sh": {"sh"}, "t": {"t"}, "th": {"th"}, "v": {"v"},
	"w": {"w"}, "y": {"y"}, "z": {"z"}, "zh": {"zh"},
}

func normalizeARPAbet(value string) string {
	return strings.TrimRight(strings.ToLower(strings.TrimSpace(value)), "0123456789")
}

func latinWords(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && r != '\''
	})
}

func normalizePinyin(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimRight(value, "12345")
	value = strings.ReplaceAll(value, "u:", "v")
	value = strings.ReplaceAll(value, "ü", "v")
	return value
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

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}
