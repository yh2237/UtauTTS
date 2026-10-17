package refspeech

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"unicode"
)

// misakiの米語G2P（Kokoroの学習に使われた音素の書き方）のうち、辞書の引き方と語の前後で変わる読みを移したもの。
// 品詞を使う分岐は、品詞が分からないときの既定値にしている。

const (
	primaryStress   = 'ˈ'
	secondaryStress = 'ˌ'
	misakiVowels    = "AIOQWYaiuæɑɒɔəɛɜɪʊʌᵻ"
	misakiConsonant = "bdfhjklmnpstvwzðŋɡɹɾʃʒʤʧθ"
	misakiPunct     = ";:,.!?—…"
	usTaus          = "AIOWYiuæɑəɛɪɹʊʌ"
)

type lexiconEntry struct {
	value    *string
	variants map[string]*string
}

// Lexiconはmisakiのus_gold.jsonとus_silver.json。
type Lexicon struct {
	gold, silver map[string]lexiconEntry
}

func LoadLexicon(goldPath, silverPath string) (*Lexicon, error) {
	gold, err := readLexicon(goldPath)
	if err != nil {
		return nil, err
	}
	silver, err := readLexicon(silverPath)
	if err != nil {
		return nil, err
	}
	return &Lexicon{gold: gold, silver: silver}, nil
}

func readLexicon(path string) (map[string]lexiconEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	entries := make(map[string]lexiconEntry, len(raw))
	for word, value := range raw {
		var entry lexiconEntry
		if len(value) > 0 && value[0] == '{' {
			if err := json.Unmarshal(value, &entry.variants); err != nil {
				return nil, err
			}
		} else if err := json.Unmarshal(value, &entry.value); err != nil {
			return nil, err
		}
		entries[word] = entry
	}
	grown := make(map[string]lexiconEntry, len(entries)*2)
	for word, entry := range entries {
		if len([]rune(word)) < 2 {
			continue
		}
		if word == strings.ToLower(word) {
			if word != capitalize(word) {
				grown[capitalize(word)] = entry
			}
		} else if word == capitalize(strings.ToLower(word)) {
			grown[strings.ToLower(word)] = entry
		}
	}
	for word, entry := range entries {
		grown[word] = entry
	}
	return grown, nil
}

func capitalize(word string) string {
	runes := []rune(strings.ToLower(word))
	if len(runes) > 0 {
		runes[0] = unicode.ToUpper(runes[0])
	}
	return string(runes)
}

// wordContextは右側の語から分かる情報。futureVowelは次の音が母音ならtrue、子音ならfalse、句読点や文末ならnil。
type wordContext struct {
	futureVowel *bool
	futureTo    bool
}

func (c wordContext) next(word, phonemes string) wordContext {
	result := wordContext{futureVowel: c.futureVowel, futureTo: word == "to" || word == "To" || word == "TO"}
	for _, r := range phonemes {
		switch {
		case strings.ContainsRune(misakiPunct, r):
			result.futureVowel = nil
		case strings.ContainsRune(misakiVowels, r):
			vowel := true
			result.futureVowel = &vowel
		case strings.ContainsRune(misakiConsonant, r):
			vowel := false
			result.futureVowel = &vowel
		default:
			continue
		}
		break
	}
	return result
}

// Wordは語の音素列を返す。辞書で読めない語はfalseを返す。
func (l *Lexicon) Word(word string, context wordContext) (string, bool) {
	word = strings.NewReplacer("‘", "'", "’", "'").Replace(word)
	var stress *float64
	if word != strings.ToLower(word) {
		value := 0.5
		if word == strings.ToUpper(word) {
			value = 2
		}
		stress = &value
	}
	return l.word(word, stress, context)
}

func (l *Lexicon) word(word string, stress *float64, context wordContext) (string, bool) {
	if phonemes, ok := l.specialCase(word, stress, context); ok {
		return phonemes, true
	}
	lower := strings.ToLower(word)
	if len([]rune(word)) > 1 && isAlpha(strings.ReplaceAll(word, "'", "")) && word != lower &&
		!l.has(word) && (word == strings.ToUpper(word) || word[1:] == strings.ToLower(word[1:])) {
		if l.has(lower) || l.stemmed(lower, stress, context) != "" {
			word = lower
		}
	}
	if l.known(word) {
		return l.lookup(word, stress, context)
	}
	if strings.HasSuffix(word, "s'") && l.known(word[:len(word)-2]+"'s") {
		return l.lookup(word[:len(word)-2]+"'s", stress, context)
	}
	if strings.HasSuffix(word, "'") && l.known(word[:len(word)-1]) {
		return l.lookup(word[:len(word)-1], stress, context)
	}
	if phonemes := l.stemmed(word, stress, context); phonemes != "" {
		return phonemes, true
	}
	return "", false
}

func (l *Lexicon) stemmed(word string, stress *float64, context wordContext) string {
	if phonemes := l.stemS(word, stress, context); phonemes != "" {
		return phonemes
	}
	if phonemes := l.stemEd(word, stress, context); phonemes != "" {
		return phonemes
	}
	ingStress := stress
	if ingStress == nil {
		half := 0.5
		ingStress = &half
	}
	return l.stemIng(word, ingStress, context)
}

func (l *Lexicon) specialCase(word string, stress *float64, context wordContext) (string, bool) {
	vowelNext := context.futureVowel != nil && *context.futureVowel
	switch word {
	case "a", "A":
		return "ɐ", true
	case "am", "Am", "AM":
		if context.futureVowel == nil || word != "am" || (stress != nil && *stress > 0) {
			return l.lookup("am", nil, context)
		}
		return "ɐm", true
	case "an", "An", "AN":
		return "ɐn", true
	case "I":
		return string(secondaryStress) + "I", true
	case "to", "To", "TO":
		if context.futureVowel == nil {
			return l.lookup("to", nil, context)
		}
		if vowelNext {
			return "tʊ", true
		}
		return "tə", true
	case "in", "In", "IN":
		if context.futureVowel == nil {
			return string(primaryStress) + "ɪn", true
		}
		return "ɪn", true
	case "the", "The", "THE":
		if vowelNext {
			return "ði", true
		}
		return "ðə", true
	case "used", "Used", "USED":
		if entry, ok := l.gold["used"]; ok && entry.variants != nil {
			key := "DEFAULT"
			if context.futureTo {
				key = "VBD"
			}
			if value := entry.variants[key]; value != nil {
				return *value, true
			}
		}
	}
	return "", false
}

func (l *Lexicon) has(word string) bool {
	_, gold := l.gold[word]
	_, silver := l.silver[word]
	return gold || silver
}

func (l *Lexicon) known(word string) bool {
	if l.has(word) {
		return true
	}
	if !isAlpha(word) || !lexiconLetters(word) {
		return false
	}
	if len(word) == 1 {
		return true
	}
	if word == strings.ToUpper(word) {
		if _, ok := l.gold[strings.ToLower(word)]; ok {
			return true
		}
	}
	return word[1:] == strings.ToUpper(word[1:])
}

func (l *Lexicon) lookup(word string, stress *float64, context wordContext) (string, bool) {
	if word == strings.ToUpper(word) {
		if _, ok := l.gold[word]; !ok {
			word = strings.ToLower(word)
		}
	}
	entry, ok := l.gold[word]
	if !ok {
		entry, ok = l.silver[word]
	}
	if !ok {
		return "", false
	}
	value := entry.value
	if entry.variants != nil {
		key := "DEFAULT"
		if _, hasNone := entry.variants["None"]; hasNone && context.futureVowel == nil {
			key = "None"
		}
		value = entry.variants[key]
	}
	if value == nil {
		return "", false
	}
	return applyStress(*value, stress), true
}

func (l *Lexicon) stemS(word string, stress *float64, context wordContext) string {
	if len(word) < 3 || !strings.HasSuffix(word, "s") {
		return ""
	}
	var stem string
	switch {
	case !strings.HasSuffix(word, "ss") && l.known(word[:len(word)-1]):
		stem = word[:len(word)-1]
	case (strings.HasSuffix(word, "'s") || (len(word) > 4 && strings.HasSuffix(word, "es") && !strings.HasSuffix(word, "ies"))) && l.known(word[:len(word)-2]):
		stem = word[:len(word)-2]
	case len(word) > 4 && strings.HasSuffix(word, "ies") && l.known(word[:len(word)-3]+"y"):
		stem = word[:len(word)-3] + "y"
	default:
		return ""
	}
	phonemes, ok := l.lookup(stem, stress, context)
	if !ok || phonemes == "" {
		return ""
	}
	last := lastRune(phonemes)
	switch {
	case strings.ContainsRune("ptkfθ", last):
		return phonemes + "s"
	case strings.ContainsRune("szʃʒʧʤ", last):
		return phonemes + "ᵻz"
	}
	return phonemes + "z"
}

func (l *Lexicon) stemEd(word string, stress *float64, context wordContext) string {
	if len(word) < 4 || !strings.HasSuffix(word, "d") {
		return ""
	}
	var stem string
	switch {
	case !strings.HasSuffix(word, "dd") && l.known(word[:len(word)-1]):
		stem = word[:len(word)-1]
	case len(word) > 4 && strings.HasSuffix(word, "ed") && !strings.HasSuffix(word, "eed") && l.known(word[:len(word)-2]):
		stem = word[:len(word)-2]
	default:
		return ""
	}
	phonemes, ok := l.lookup(stem, stress, context)
	if !ok || phonemes == "" {
		return ""
	}
	runes := []rune(phonemes)
	last := runes[len(runes)-1]
	switch {
	case strings.ContainsRune("pkfθʃsʧ", last):
		return phonemes + "t"
	case last == 'd':
		return phonemes + "ᵻd"
	case last != 't':
		return phonemes + "d"
	case len(runes) < 2:
		return phonemes + "ɪd"
	case strings.ContainsRune(usTaus, runes[len(runes)-2]):
		return string(runes[:len(runes)-1]) + "ɾᵻd"
	}
	return phonemes + "ᵻd"
}

func (l *Lexicon) stemIng(word string, stress *float64, context wordContext) string {
	if len(word) < 5 || !strings.HasSuffix(word, "ing") {
		return ""
	}
	base := word[:len(word)-3]
	var stem string
	switch {
	case len(word) > 5 && l.known(base):
		stem = base
	case l.known(base + "e"):
		stem = base + "e"
	case len(word) > 5 && doubledIng(word) && l.known(word[:len(word)-4]):
		stem = word[:len(word)-4]
	default:
		return ""
	}
	phonemes, ok := l.lookup(stem, stress, context)
	if !ok || phonemes == "" {
		return ""
	}
	runes := []rune(phonemes)
	if len(runes) > 1 && runes[len(runes)-1] == 't' && strings.ContainsRune(usTaus, runes[len(runes)-2]) {
		return string(runes[:len(runes)-1]) + "ɾɪŋ"
	}
	return phonemes + "ɪŋ"
}

func doubledIng(word string) bool {
	if strings.HasSuffix(word, "cking") {
		return true
	}
	runes := []rune(word)
	if len(runes) < 5 {
		return false
	}
	a, b := runes[len(runes)-5], runes[len(runes)-4]
	return a == b && strings.ContainsRune("bcdgklmnprstvxz", a)
}

// applyStressはmisakiのapply_stress。stressがnilなら変えない。
func applyStress(phonemes string, stress *float64) string {
	if stress == nil {
		return phonemes
	}
	s := *stress
	hasPrimary := strings.ContainsRune(phonemes, primaryStress)
	hasSecondary := strings.ContainsRune(phonemes, secondaryStress)
	hasVowel := strings.ContainsAny(phonemes, misakiVowels)
	switch {
	case s < -1:
		return strings.NewReplacer(string(primaryStress), "", string(secondaryStress), "").Replace(phonemes)
	case s == -1 || ((s == 0 || s == -0.5) && hasPrimary):
		phonemes = strings.ReplaceAll(phonemes, string(secondaryStress), "")
		return strings.ReplaceAll(phonemes, string(primaryStress), string(secondaryStress))
	case (s == 0 || s == 0.5 || s == 1) && !hasPrimary && !hasSecondary:
		if !hasVowel {
			return phonemes
		}
		return restress(string(secondaryStress) + phonemes)
	case s >= 1 && !hasPrimary && hasSecondary:
		return strings.ReplaceAll(phonemes, string(secondaryStress), string(primaryStress))
	case s > 1 && !hasPrimary && !hasSecondary:
		if !hasVowel {
			return phonemes
		}
		return restress(string(primaryStress) + phonemes)
	}
	return phonemes
}

// restressは強勢の記号を、その後ろで最初の母音の直前へ移す。
func restress(phonemes string) string {
	runes := []rune(phonemes)
	type item struct {
		position float64
		r        rune
	}
	items := make([]item, len(runes))
	for i, r := range runes {
		items[i] = item{float64(i), r}
		if r == primaryStress || r == secondaryStress {
			for j := i; j < len(runes); j++ {
				if strings.ContainsRune(misakiVowels, runes[j]) {
					items[i].position = float64(j) - 0.5
					break
				}
			}
		}
	}
	sort.SliceStable(items, func(a, b int) bool { return items[a].position < items[b].position })
	var b strings.Builder
	for _, it := range items {
		b.WriteRune(it.r)
	}
	return b.String()
}

func isAlpha(word string) bool {
	if word == "" {
		return false
	}
	for _, r := range word {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

func lexiconLetters(word string) bool {
	for _, r := range word {
		if !(r == '\'' || r == '-' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
			return false
		}
	}
	return true
}

func lastRune(value string) rune {
	runes := []rune(value)
	return runes[len(runes)-1]
}
