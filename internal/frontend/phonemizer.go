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

	PhonemizerJapanese = "ja-kana"
	PhonemizerEnglish  = "en-arpasing"
	PhonemizerChinese  = "zh-cvvc"
)

func ResolveLanguage(language, phonemizer string) (string, string, error) {
	language = strings.ToLower(strings.TrimSpace(language))
	phonemizer = strings.ToLower(strings.TrimSpace(phonemizer))
	if language == "" {
		switch phonemizer {
		case PhonemizerEnglish:
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
	want := map[string]string{
		LanguageJapanese: PhonemizerJapanese,
		LanguageEnglish:  PhonemizerEnglish,
		LanguageChinese:  PhonemizerChinese,
	}[language]
	if want == "" {
		return "", "", fmt.Errorf("unsupported language %q", language)
	}
	if phonemizer != want {
		return "", "", fmt.Errorf("phonemizer %q does not support language %q", phonemizer, language)
	}
	return language, phonemizer, nil
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
				return "", nil, fmt.Errorf("English dictionary has no pronunciation for %q; specify --reading in ARPAbet", word)
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
