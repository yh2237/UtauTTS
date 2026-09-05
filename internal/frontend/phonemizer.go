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
	return ParseEnglishDeltaWithConfig(text, reading, dictionary, PresampConfig{})
}

func ParseEnglishDeltaWithConfig(text, reading string, dictionary map[string]string, config PresampConfig) (string, []Mora, error) {
	return parseEnglishSyllables(text, reading, dictionary, deltaEnglishSymbols, " ", true, config)
}

// ParseEnglishVCCVはARPAbet読みをCz式VCCVへ変換する。
func ParseEnglishVCCV(text, reading string, dictionary map[string]string) (string, []Mora, error) {
	return parseEnglishSyllables(text, reading, dictionary, vccvEnglishSymbols, " ", false, PresampConfig{})
}

type PresampConfig struct {
	Vowels       map[string]string
	Consonants   map[string]string
	Replacements map[string]string
	Endings      []string
}

func parseEnglishSyllables(text, reading string, dictionary map[string]string, symbols map[string][]string, separator string, spacedStart bool, config PresampConfig) (string, []Mora, error) {
	pronunciation, words, err := englishPronunciation(text, reading, dictionary)
	if err != nil {
		return "", nil, err
	}
	var syllableWords [][]englishSyllable
	for _, word := range words {
		if len(word) == 1 && word[0] == "SP" {
			syllableWords = append(syllableWords, nil)
			continue
		}
		syllables, syllableErr := syllabifyEnglishWord(word, symbols)
		if syllableErr != nil {
			return "", nil, syllableErr
		}
		if len(syllables) == 0 {
			if len(syllableWords) == 0 || len(syllableWords[len(syllableWords)-1]) == 0 {
				return "", nil, fmt.Errorf("ARPAbet reading contains no vowel")
			}
			lastWord := syllableWords[len(syllableWords)-1]
			lastWord[len(lastWord)-1].coda = append(lastWord[len(lastWord)-1].coda, normalizeEnglishPhones(word)...)
			continue
		}
		syllableWords = append(syllableWords, syllables)
	}
	if len(syllableWords) == 0 {
		return "", nil, fmt.Errorf("ARPAbet reading contains no vowel")
	}
	units := make([]Mora, 0)
	previousVowels := []string(nil)
	phraseStart := true
	for wordIndex, syllables := range syllableWords {
		if len(syllables) == 0 {
			if len(units) > 0 && !units[len(units)-1].Pause {
				units = append(units, Mora{Pause: true})
			}
			previousVowels, phraseStart = nil, true
			continue
		}
		for syllableIndex, syllable := range syllables {
			atPhraseStart := phraseStart
			mainOnset := syllable.onset
			if syllableIndex > 0 && len(syllable.onset) == 0 && len(syllables[syllableIndex-1].coda) > 0 {
				bridgeOnset := append([]string{syllables[syllableIndex-1].coda[len(syllables[syllableIndex-1].coda)-1]}, syllable.onset...)
				mainOnset = bridgeOnset
			}
			main := englishMainAliases(mainOnset, symbols[syllable.vowel], symbols)
			if !sameStrings(mainOnset, syllable.onset) {
				main = append(main, englishMainAliases(syllable.onset, symbols[syllable.vowel], symbols)...)
			}
			if atPhraseStart {
				prefix := "-"
				if spacedStart {
					prefix = "- "
				}
				prefixed := make([]string, 0, len(main))
				for _, alias := range main {
					prefixed = append(prefixed, prefix+alias)
				}
				main = append(prefixed, main...)
			}
			phraseStart = false
			var transitions []string
			if syllableIndex == 0 || len(syllables[syllableIndex-1].coda) == 0 {
				transitions = combineEnglishTransitionAliases(previousVowels, syllable.onset, symbols, separator)
			}
			if len(syllable.onset) > 1 {
				transitions = append(transitions, englishOnsetClusterAliases(syllable.onset, symbols, separator, atPhraseStart)...)
			}
			main = uniqueStrings(main)
			units = append(units, Mora{
				Text: main[0], Consonant: strings.Join(syllable.onset, " "), Vowel: symbols[syllable.vowel][0], Stress: syllable.stress,
				Aliases: &AliasHints{Main: main, MainKinds: repeatAliasKind("cv", len(main)), Transition: uniqueStrings(transitions)},
			})
			current := &units[len(units)-1]
			previousVowels = symbols[syllable.vowel]
			lastSyllable := syllableIndex+1 == len(syllables)
			lastWord := wordIndex+1 == len(syllableWords) || len(syllableWords[wordIndex+1]) == 0
			if len(syllable.coda) > 0 {
				if lastSyllable {
					current.Aliases.Endings = englishTerminalConsonants(previousVowels, syllable.coda, symbols, separator)
					phraseStart = !lastWord
					if phraseStart {
						previousVowels = nil
					}
				} else {
					current.Aliases.Endings = englishSyllableBridge(previousVowels, syllable.coda, syllables[syllableIndex+1].onset, symbols, separator)
				}
			} else if lastSyllable && lastWord {
				current.Aliases.Endings = [][]string{englishEndingAliases(previousVowels, config)}
			}
		}
	}
	return pronunciation, units, nil
}

type englishSyllable struct {
	onset  []string
	vowel  string
	coda   []string
	stress int
}

func syllabifyEnglishWord(raw []string, symbols map[string][]string) ([]englishSyllable, error) {
	phones := normalizeEnglishPhones(raw)
	for _, phone := range phones {
		if len(symbols[phone]) == 0 {
			return nil, fmt.Errorf("unsupported ARPAbet phoneme %q", phone)
		}
	}
	var vowels []int
	for i, phone := range phones {
		if englishVowels[phone] {
			vowels = append(vowels, i)
		}
	}
	if len(vowels) == 0 {
		return nil, nil
	}
	syllables := make([]englishSyllable, len(vowels))
	for i, vowelAt := range vowels {
		syllables[i].vowel = phones[vowelAt]
		syllables[i].stress = arpabetStress(raw[vowelAt])
		if i == 0 {
			syllables[i].onset = append([]string(nil), phones[:vowelAt]...)
			continue
		}
		cluster := phones[vowels[i-1]+1 : vowelAt]
		onsetCount := englishOnsetSuffixLength(cluster)
		syllables[i-1].coda = append([]string(nil), cluster[:len(cluster)-onsetCount]...)
		syllables[i].onset = append([]string(nil), cluster[len(cluster)-onsetCount:]...)
	}
	syllables[len(syllables)-1].coda = append([]string(nil), phones[vowels[len(vowels)-1]+1:]...)
	return syllables, nil
}

func normalizeEnglishPhones(raw []string) []string {
	result := make([]string, 0, len(raw))
	for _, phone := range raw {
		if normalized := normalizeARPAbet(phone); normalized != "" {
			result = append(result, normalized)
		}
	}
	return result
}

func englishOnsetSuffixLength(cluster []string) int {
	for count := len(cluster); count > 0; count-- {
		if validEnglishOnset(cluster[len(cluster)-count:]) {
			return count
		}
	}
	return 0
}

func validEnglishOnset(cluster []string) bool {
	if len(cluster) == 1 {
		return cluster[0] != "ng"
	}
	return englishOnsets[strings.Join(cluster, " ")]
}

var englishOnsets = map[string]bool{
	"b l": true, "b r": true, "d r": true, "d w": true, "f l": true, "f r": true,
	"g l": true, "g r": true, "g w": true, "k l": true, "k r": true, "k w": true,
	"p l": true, "p r": true, "sh r": true, "s f": true, "s k": true, "s l": true,
	"s m": true, "s n": true, "s p": true, "s t": true, "s w": true, "t r": true,
	"t w": true, "th r": true, "th w": true, "v r": true,
	"s k l": true, "s k r": true, "s k w": true, "s p l": true, "s p r": true,
	"s t r": true,
}

func englishEndingAliases(vowels []string, config PresampConfig) []string {
	var result []string
	for _, vowel := range vowels {
		for _, format := range config.Endings {
			result = append(result, strings.ReplaceAll(format, "%v%", vowel))
		}
		result = append(result, vowel+" -", vowel+"-")
	}
	return uniqueStrings(result)
}

func englishTerminalConsonants(vowels, coda []string, symbols map[string][]string, separator string) [][]string {
	cluster := combineEnglishAliases(coda, []string{""}, symbols)
	if len(coda) == 1 {
		var endings []string
		for _, vowel := range vowels {
			for _, value := range cluster {
				endings = append(endings, vowel+separator+value+"-", vowel+value+"-")
			}
		}
		return [][]string{uniqueStrings(endings)}
	}
	first := combineEnglishAliases(coda[:1], []string{""}, symbols)
	rest := combineEnglishAliases(coda[1:], []string{""}, symbols)
	var firstAliases, restAliases []string
	for _, vowel := range vowels {
		for _, value := range first {
			firstAliases = append(firstAliases, vowel+separator+value, vowel+value)
		}
	}
	for _, left := range first {
		for _, right := range rest {
			restAliases = append(restAliases, left+separator+right+"-", left+right+"-")
		}
	}
	return [][]string{uniqueStrings(firstAliases), uniqueStrings(restAliases)}
}

func englishSyllableBridge(vowels, coda, nextOnset []string, symbols map[string][]string, separator string) [][]string {
	first := combineEnglishAliases(coda[:1], []string{""}, symbols)
	var firstAliases []string
	for _, vowel := range vowels {
		for _, consonant := range first {
			firstAliases = append(firstAliases, vowel+separator+consonant, vowel+consonant)
		}
	}
	result := [][]string{uniqueStrings(firstAliases)}
	restPhones := append(append([]string(nil), coda[1:]...), nextOnset...)
	if len(restPhones) == 0 {
		return result
	}
	rest := combineEnglishAliases(restPhones, []string{""}, symbols)
	var bridge []string
	for _, left := range first {
		for _, right := range rest {
			bridge = append(bridge, left+separator+right, left+right)
		}
	}
	return append(result, uniqueStrings(bridge))
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func englishPronunciation(text, reading string, dictionary map[string]string) (string, [][]string, error) {
	pronunciation := strings.TrimSpace(reading)
	var result [][]string
	if pronunciation != "" {
		for _, part := range strings.Split(pronunciation, "|") {
			fields := strings.Fields(part)
			if len(fields) > 0 {
				result = append(result, fields)
			}
		}
	} else {
		words := latinWords(text)
		if len(words) == 0 {
			return "", nil, fmt.Errorf("English text contains no words")
		}
		parts := make([]string, 0, len(words))
		for _, word := range words {
			if word == "<pause>" {
				result = append(result, []string{"SP"})
				parts = append(parts, "SP")
				continue
			}
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
			fields := strings.Fields(value)
			if len(fields) > 0 {
				result = append(result, fields)
				parts = append(parts, strings.Join(fields, " "))
			}
		}
		pronunciation = strings.Join(parts, " | ")
	}
	if len(result) == 0 {
		return "", nil, fmt.Errorf("ARPAbet reading is empty")
	}
	return pronunciation, result, nil
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

func englishMainAliases(onset []string, vowels []string, symbols map[string][]string) []string {
	if len(onset) == 0 {
		return combineEnglishAliases(nil, vowels, symbols)
	}
	var result []string
	for start := 0; start < len(onset); start++ {
		result = append(result, combineEnglishAliases(onset[start:], vowels, symbols)...)
	}
	return uniqueStrings(result)
}

func englishOnsetClusterAliases(onset []string, symbols map[string][]string, separator string, phraseStart bool) []string {
	clusters := combineEnglishAliases(onset, []string{""}, symbols)
	var result []string
	for _, cluster := range clusters {
		if phraseStart {
			result = append(result, "- "+cluster, "-"+cluster)
		}
		result = append(result, cluster)
	}
	first := combineEnglishAliases(onset[:1], []string{""}, symbols)
	rest := combineEnglishAliases(onset[1:], []string{""}, symbols)
	for _, left := range first {
		for _, right := range rest {
			result = append(result, left+separator+right, left+right)
		}
	}
	return uniqueStrings(result)
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
	pronunciation, words, err := englishPronunciation(text, reading, dictionary)
	if err != nil {
		return "", nil, err
	}
	var morae []Mora
	previous := "-"
	for _, word := range words {
		if len(word) == 1 && word[0] == "SP" {
			if len(morae) > 0 && !morae[len(morae)-1].Pause {
				last := &morae[len(morae)-1]
				last.Aliases.Endings = [][]string{{last.Text + " -", last.Text + "-"}}
				morae = append(morae, Mora{Pause: true})
			}
			previous = "-"
			continue
		}
		for _, raw := range word {
			symbol := normalizeARPAbet(raw)
			if symbol == "" {
				continue
			}
			candidates := []string{previous + " " + symbol, symbol}
			if previous != "-" {
				candidates = append(candidates, "- "+symbol)
			}
			mora := Mora{Text: symbol, Stress: arpabetStress(raw), Aliases: &AliasHints{Main: uniqueStrings(candidates)}}
			if englishVowels[symbol] {
				mora.Vowel = symbol
				mora.DurationScale = 1
			} else {
				mora.Consonant = symbol
				mora.DurationScale = 0.45
			}
			morae = append(morae, mora)
			previous = symbol
		}
	}
	if len(morae) > 0 && !morae[len(morae)-1].Pause {
		last := morae[len(morae)-1].Text
		morae[len(morae)-1].Aliases.Endings = [][]string{{last + " -", last + "-"}}
	}
	return pronunciation, morae, nil
}

func ParseChineseCVVC(text, reading string, dictionary map[string]string) (string, []Mora, error) {
	return ParseChineseCVVCWithConfig(text, reading, dictionary, PresampConfig{})
}

func ParseChineseCVVCWithConfig(text, reading string, dictionary map[string]string, config PresampConfig) (string, []Mora, error) {
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
		initial, final := config.Consonants[syllable], config.Vowels[syllable]
		if final == "" {
			initial, final = splitPinyin(syllable)
		}
		if final == "" {
			return "", nil, fmt.Errorf("invalid Pinyin syllable %q", raw)
		}
		candidates := []string{syllable}
		kinds := []string{"cv"}
		if phraseStart {
			candidates = append([]string{"- " + syllable}, candidates...)
			kinds = append([]string{"vcv"}, kinds...)
		} else if previousFinal != "" {
			candidates = append([]string{previousFinal + " " + syllable}, candidates...)
			kinds = append([]string{"vcv"}, kinds...)
		}
		mora := Mora{Text: syllable, Consonant: initial, Vowel: final, Tone: tone, Aliases: &AliasHints{Main: candidates, MainKinds: kinds}}
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

func chineseSyllables(text string, dictionary map[string]string) ([]string, error) {
	args := pinyin.NewArgs()
	args.Style = pinyin.Tone3
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
	"aa": true, "ae": true, "ah": true, "ao": true, "aw": true, "ax": true, "ay": true,
	"eh": true, "er": true, "ey": true, "ih": true, "iy": true, "ow": true,
	"oy": true, "uh": true, "uw": true,
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

func normalizeARPAbet(value string) string {
	return strings.TrimRight(strings.ToLower(strings.TrimSpace(value)), "0123456789")
}

func arpabetStress(value string) int {
	value = strings.TrimSpace(value)
	if len(value) == 0 {
		return 0
	}
	last := value[len(value)-1]
	if last >= '0' && last <= '2' {
		return int(last - '0')
	}
	return 0
}

func latinWords(text string) []string {
	var result []string
	var word strings.Builder
	flush := func() {
		if word.Len() > 0 {
			result = append(result, word.String())
			word.Reset()
		}
	}
	for _, r := range text {
		if unicode.IsLetter(r) || r == '\'' {
			word.WriteRune(r)
			continue
		}
		flush()
		if strings.ContainsRune(",.;:!?、。！？\n", r) && len(result) > 0 && result[len(result)-1] != "<pause>" {
			result = append(result, "<pause>")
		}
	}
	flush()
	return result
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

func repeatAliasKind(kind string, count int) []string {
	result := make([]string, count)
	for index := range result {
		result[index] = kind
	}
	return result
}
