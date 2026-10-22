package frontend

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// スペイン語の読みは、Kokoroの音素記号の音節を「.」で、語を空白でつなぐ（例「ˈo.la , kˈe tˈal ?」）。
// 強勢記号は音節の母音の直前に置く。記号はmisakiがespeak-ng（es）の出力を直したものに合わせる。

const (
	esPrimary   = "ˈ"
	esSecondary = "ˌ"
)

// ParseSpanishは文か読みからスペイン語の読みとモーラ（1音節を1モーラ）を返す。
func ParseSpanish(text, reading string, dictionary map[string]string) (string, []Mora, error) {
	if strings.TrimSpace(reading) == "" {
		var err error
		reading, err = SpanishReading(text, dictionary)
		if err != nil {
			return "", nil, err
		}
	}
	morae, err := syllableMorae(reading, LanguageSpanish)
	if err != nil {
		return "", nil, err
	}
	return reading, morae, nil
}

// IsSyllablePauseは、音節で読む言語の読みの語が休止の記号かを返す。
func IsSyllablePause(field string) bool { return spanishPauses[field] != "" }

// syllableLanguagesは、音節を1モーラにして参照照合だけで読む言語と、その発音形式。
var syllableLanguages = map[string]string{
	LanguageSpanish:    PhonemizerSpanish,
	LanguagePortuguese: PhonemizerPortuguese,
}

// IsSyllableLanguageは、音節を1モーラにして参照照合だけで読む言語かを返す。
func IsSyllableLanguage(language string) bool { return syllableLanguages[language] != "" }

// ParseSyllableLanguageは、音節で読む言語の文か読みから、読みとモーラを返す。
func ParseSyllableLanguage(language, phonemizer, text, reading string, dictionary map[string]string) (string, []Mora, error) {
	if syllableLanguages[language] != phonemizer {
		return "", nil, fmt.Errorf("unsupported phonemizer %q for language %q", phonemizer, language)
	}
	if language == LanguagePortuguese {
		return ParsePortuguese(text, reading, dictionary)
	}
	return ParseSpanish(text, reading, dictionary)
}

// spanishPausesは読みのうち、休止を表す記号と休止の種類。
var spanishPauses = map[string]string{
	",": PauseKindComma, ";": PauseKindComma, ":": PauseKindComma,
	".": PauseKindPeriod, "?": PauseKindQuestion, "!": PauseKindPeriod, "…": PauseKindEllipsis,
}

// syllableMoraeは、音節を「.」で、語を空白で区切った読みをモーラにする。文末の記号はモーラにしない。
func syllableMorae(reading, language string) ([]Mora, error) {
	fields := strings.Fields(reading)
	last := len(fields) - 1
	for last >= 0 && spanishPauses[fields[last]] != "" {
		last--
	}
	var morae []Mora
	word := 0
	for i, field := range fields[:last+1] {
		if kind := spanishPauses[field]; kind != "" {
			if len(morae) > 0 && !morae[len(morae)-1].Pause {
				morae = append(morae, Mora{Language: language, Text: field, Pause: true, PauseKind: kind})
			}
			continue
		}
		syllables := strings.Split(field, ".")
		for j, syllable := range syllables {
			plain, stress := spanishStripStress(syllable)
			if plain == "" {
				return nil, fmt.Errorf("empty syllable in %q", field)
			}
			onset, rest := spanishSplitOnset(plain)
			morae = append(morae, Mora{
				Language: language, WordIndex: word, WordEnd: j == len(syllables)-1,
				Text: plain, Consonant: onset, Vowel: rest, Stress: stress, StressKnown: true,
			})
		}
		if i < last {
			word++
		}
	}
	if len(morae) == 0 {
		return nil, fmt.Errorf("no syllables in %q", reading)
	}
	return morae, nil
}

func spanishStripStress(syllable string) (string, int) {
	stress := 0
	if strings.Contains(syllable, esPrimary) {
		stress = 1
	} else if strings.Contains(syllable, esSecondary) {
		stress = 2
	}
	return strings.NewReplacer(esPrimary, "", esSecondary, "").Replace(syllable), stress
}

// spanishSplitOnsetは音節を、最初の母音の前の子音と残りに分ける。
func spanishSplitOnset(syllable string) (string, string) {
	for i, r := range syllable {
		if strings.ContainsRune(syllableNucleusRunes, r) {
			return syllable[:i], syllable[i:]
		}
	}
	return syllable, ""
}

// esNucleusRunesは音節の核になる記号（I=ai、W=au、A=eiはmisakiの二重母音）。
const esNucleusRunes = "aeiouɛIWA"

// syllableNucleusRunesは、ほかの言語も含めた音節の核の記号。
const syllableNucleusRunes = "aeiouɛɔæɐəAIWYOʊyũõĩ"

// esUnstressedは、2語以上の文で強勢を置かない機能語（espeak-ngの扱いに合わせる）。
var esUnstressed = map[string]bool{
	"a": true, "al": true, "de": true, "del": true, "el": true, "la": true, "las": true, "lo": true,
	"los": true, "le": true, "les": true, "me": true, "te": true, "se": true, "nos": true, "os": true,
	"mi": true, "mis": true, "tu": true, "tus": true, "su": true, "sus": true, "que": true, "e": true,
	"o": true, "u": true, "en": true, "con": true, "por": true, "si": true,
}

// esSecondaryWordsは、文の中で第2強勢になる語。
var esSecondaryWords = map[string]bool{
	"para": true, "porque": true, "cuando": true, "aunque": true, "sobre": true, "desde": true, "como": true,
	"pero": true, "hasta": true, "entre": true, "donde": true, "mientras": true, "hacia": true,
	"nuestro": true, "nuestra": true, "nuestros": true, "nuestras": true,
}

// esWordExceptionsは規則に合わない語の読み（音節を「.」で区切る）。
var esWordExceptions = map[string]string{
	"méxico": "mˈɛ.xi.ko", "mexicano": "mˌe.xi.kˈa.no", "texas": "tˈe.xas", "oaxaca": "wa.xˈa.ka",
	"xavier": "sa.βjˈeɾ",
}

// esTokenは文を区切った語か記号。
type esToken struct {
	word  string
	pause string
	// joinNextは、次の語とハイフンでつながっているか。
	joinNext bool
}

// SpanishReadingは文をスペイン語の読みにする。dictionaryの語（小文字）は、その読みを使う。
func SpanishReading(text string, dictionary map[string]string) (string, error) {
	tokens := spanishTokens(text)
	words := 0
	for _, token := range tokens {
		if token.word != "" {
			words++
		}
	}
	if words == 0 {
		return "", fmt.Errorf("no Spanish words in %q", text)
	}
	var fields []string
	// prevは直前の音（語や記号をまたいで、閉鎖音か摩擦音かを決める）。
	prev := ""
	for _, token := range tokens {
		if token.pause != "" {
			if len(fields) > 0 {
				fields = append(fields, token.pause)
			}
			prev = ""
			continue
		}
		if value := dictionary[token.word]; value != "" {
			fields = append(fields, value)
			prev = esLastPhone(value)
			continue
		}
		if value := esWordExceptions[token.word]; value != "" {
			fields = append(fields, value)
			prev = esLastPhone(value)
			continue
		}
		parsed, err := spanishWord(token.word)
		if err != nil {
			return "", err
		}
		if words > 1 {
			switch {
			case esUnstressed[token.word]:
				for i := range parsed {
					parsed[i].stress = 0
				}
			case esSecondaryWords[token.word]:
				for i := range parsed {
					if parsed[i].stress == 1 {
						parsed[i].stress = 2
					}
				}
			}
		}
		if token.word == "y" {
			parsed[0].stress = 0
		}
		text := esRender(parsed, prev)
		fields = append(fields, text)
		prev = esLastPhone(text)
	}
	esAssimilateAcrossWords(fields)
	return strings.Join(fields, " "), nil
}

// esAssimilateAcrossWordsは、語末のnを次の語の頭の音に合わせる（un beso → ˈum bˈeso、un gato → ˈuŋ ɡˈato）。jの前では同化しない。
func esAssimilateAcrossWords(fields []string) {
	for i := 0; i+1 < len(fields); i++ {
		if spanishPauses[fields[i+1]] != "" {
			continue
		}
		next := []rune(strings.NewReplacer(esPrimary, "", esSecondary, "").Replace(fields[i+1]))
		if len(next) == 0 {
			continue
		}
		if strings.HasSuffix(fields[i], "d") && strings.ContainsRune(esNucleusRunes, next[0]) {
			fields[i] = strings.TrimSuffix(fields[i], "d") + "ð"
			continue
		}
		if !strings.HasSuffix(fields[i], "n") {
			continue
		}
		switch next[0] {
		case 'b', 'p':
			fields[i] = strings.TrimSuffix(fields[i], "n") + "m"
		case 'ɡ':
			fields[i] = strings.TrimSuffix(fields[i], "n") + "ŋ"
		}
	}
}

func esLastPhone(reading string) string {
	runes := []rune(strings.NewReplacer(esPrimary, "", esSecondary, "", ".", "").Replace(reading))
	if len(runes) == 0 {
		return ""
	}
	return string(runes[len(runes)-1])
}

func spanishTokens(text string) []esToken {
	text = norm.NFC.String(strings.ToLower(text))
	var tokens []esToken
	var word strings.Builder
	flush := func() {
		if word.Len() > 0 {
			tokens = append(tokens, esToken{word: word.String()})
			word.Reset()
		}
	}
	for _, r := range text {
		switch {
		case unicode.IsLetter(r):
			word.WriteRune(r)
		case r == '\'' || r == '’':
		case unicode.IsDigit(r):
			word.WriteRune(r)
		default:
			flush()
			if pause := esPauseSymbol(r); pause != "" {
				if len(tokens) > 0 && tokens[len(tokens)-1].pause != "" {
					continue
				}
				tokens = append(tokens, esToken{pause: pause})
			}
		}
	}
	flush()
	var expanded []esToken
	for _, token := range tokens {
		if token.word != "" && strings.IndexFunc(token.word, unicode.IsDigit) >= 0 {
			for _, number := range strings.Fields(spanishNumberWords(token.word)) {
				expanded = append(expanded, esToken{word: number})
			}
			continue
		}
		expanded = append(expanded, token)
	}
	return expanded
}

func esPauseSymbol(r rune) string {
	switch r {
	case ',', '、':
		return ","
	case ';':
		return ";"
	case ':':
		return ":"
	case '.', '。':
		return "."
	case '?', '？':
		return "?"
	case '!', '！':
		return "!"
	case '…':
		return "…"
	}
	return ""
}

// esPhoneは語の中の1つの音。vowelなら母音の文字（強勢の判定に使う）。
type esPhone struct {
	symbol string
	vowel  bool
	// accentは綴りに強勢符号がある母音。
	accent bool
	// strongはa、e、o、または強勢符号のあるi、u。
	strong bool
	// rollはrの震え音（語頭、rr、l・n・sの後）。
	roll bool
	// splitは、前の母音と二重母音にならない母音（üの後、hの後）。
	split bool
	// fromYは語末のyから来たi（muy → mˈuj）。
	fromY bool
}

// esSyllableは1つの音節。
type esSyllable struct {
	onset   []esPhone
	nucleus []esPhone
	coda    []esPhone
	stress  int
	// closedEは、強勢があってもeを広くしない音節（-mente）。
	closedE bool
}

// spanishWordは1語を音節に分け、強勢を決める。
func spanishWord(word string) ([]esSyllable, error) {
	phones, err := esLetters(word)
	if err != nil {
		return nil, err
	}
	syllables := esSyllabify(phones)
	if len(syllables) == 0 {
		return nil, fmt.Errorf("Spanish word %q has no vowel", word)
	}
	esAssignStress(word, syllables)
	return syllables, nil
}

// esLettersは綴りを音の並びにする。異音（b/β など）はesRenderで決める。
func esLetters(word string) ([]esPhone, error) {
	runes := []rune(word)
	var phones []esPhone
	at := func(i int) rune {
		if i < 0 || i >= len(runes) {
			return 0
		}
		return runes[i]
	}
	front := func(r rune) bool { return strings.ContainsRune("eéií", r) }
	consonant := func(symbol string) { phones = append(phones, esPhone{symbol: symbol}) }
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch r {
		case 'a', 'e', 'i', 'o', 'u', 'á', 'é', 'í', 'ó', 'ú', 'ü':
			base, accent := esVowelBase(r)
			previous := at(i - 1)
			phones = append(phones, esPhone{symbol: base, vowel: true, accent: accent,
				strong: accent || base == "a" || base == "e" || base == "o" || r == 'ü',
				split:  previous == 'ü' || (previous == 'h' && i > 1) || esAfterClusterGlide(phones)})
		case 'y':
			// 語末や子音の前のyは母音のi。
			next := at(i + 1)
			if next == 0 || !esIsVowelLetter(next) {
				if len(runes) == 1 {
					phones = append(phones, esPhone{symbol: "i", vowel: true, strong: true})
				} else {
					phones = append(phones, esPhone{symbol: "i", vowel: true, fromY: next == 0})
				}
			} else {
				consonant("ʝ")
			}
		case 'b', 'v':
			consonant("b")
		case 'c':
			switch next := at(i + 1); {
			case next == 'h':
				consonant("ʧ")
				i++
			case front(next):
				consonant("θ")
			default:
				consonant("k")
			}
		case 'd':
			consonant("d")
		case 'f':
			consonant("f")
		case 'g':
			next := at(i + 1)
			switch {
			case front(next):
				consonant("x")
			case next == 'u' && front(at(i+2)):
				consonant("ɡ")
				i++
			case next == 'n' && i == 0:
			default:
				consonant("ɡ")
			}
		case 'h':
			// hie・hueの語頭は半母音（hielo → jˈelo）。それ以外のhは読まない。
		case 'j':
			consonant("x")
		case 'k':
			consonant("k")
		case 'l':
			if at(i+1) == 'l' {
				consonant("ʎ")
				i++
			} else {
				consonant("l")
			}
		case 'm':
			consonant("m")
		case 'n':
			consonant("n")
		case 'ñ':
			consonant("ɲ")
		case 'p':
			if i == 0 && at(i+1) == 's' {
				continue
			}
			consonant("p")
		case 'q':
			consonant("k")
			if at(i+1) == 'u' {
				i++
			}
		case 'r':
			roll := i == 0 || strings.ContainsRune("lns", at(i-1))
			if at(i+1) == 'r' {
				roll = true
				i++
			}
			phones = append(phones, esPhone{symbol: "ɾ", roll: roll})
		case 's':
			consonant("s")
		case 't':
			consonant("t")
		case 'w':
			consonant("w")
		case 'x':
			if i == 0 {
				consonant("s")
			} else {
				consonant("k")
				consonant("s")
			}
		case 'z':
			consonant("θ")
		case 'ç':
			consonant("s")
		default:
			return nil, fmt.Errorf("unsupported letter %q in Spanish word %q", string(r), word)
		}
	}
	return phones, nil
}

// esAfterClusterGlideは、直前が「子音＋l・r」か語頭のrの後のi、uかを返す。
// その後の母音とは二重母音にしない（cliente → kliˈɛnte、ruido → ruˈiðo）。
func esAfterClusterGlide(phones []esPhone) bool {
	n := len(phones)
	if n < 2 {
		return false
	}
	glide := phones[n-1]
	if !glide.vowel || glide.strong || (glide.symbol != "i" && glide.symbol != "u") {
		return false
	}
	liquid := phones[n-2]
	if liquid.symbol != "l" && liquid.symbol != "ɾ" {
		return false
	}
	if n == 2 {
		return liquid.roll
	}
	return !phones[n-3].vowel
}

func esIsVowelLetter(r rune) bool { return strings.ContainsRune("aeiouáéíóúü", r) }

func esVowelBase(r rune) (string, bool) {
	switch r {
	case 'á':
		return "a", true
	case 'é':
		return "e", true
	case 'í':
		return "i", true
	case 'ó':
		return "o", true
	case 'ú':
		return "u", true
	case 'ü':
		return "u", false
	}
	return string(r), false
}

// esOnsetsは音節の頭にまとめられる子音の2つ組。
var esOnsets = map[string]bool{
	"pl": true, "pɾ": true, "bl": true, "bɾ": true, "kl": true, "kɾ": true, "ɡl": true, "ɡɾ": true,
	"fl": true, "fɾ": true, "tɾ": true, "dɾ": true,
}

func esSyllabify(phones []esPhone) []esSyllable {
	// 母音の並びを音節の核にまとめる。弱い母音（強勢符号のないi、u）は隣の母音と二重母音になる。
	type group struct{ start, end int }
	var nuclei []group
	for i := 0; i < len(phones); {
		if !phones[i].vowel {
			i++
			continue
		}
		start := i
		end := i + 1
		for end < len(phones) && phones[end].vowel {
			if (phones[end-1].strong && phones[end].strong) || phones[end].split {
				break
			}
			// 弱い母音が2つ以上続くとき（iu、ui）は、1つの核にする。
			end++
		}
		nuclei = append(nuclei, group{start, end})
		i = end
	}
	if len(nuclei) == 0 {
		return nil
	}
	syllables := make([]esSyllable, len(nuclei))
	for n, g := range nuclei {
		syllables[n].nucleus = phones[g.start:g.end]
	}
	syllables[0].onset = phones[:nuclei[0].start]
	for n := 0; n+1 < len(nuclei); n++ {
		between := phones[nuclei[n].end:nuclei[n+1].start]
		split := len(between)
		switch {
		case len(between) == 1:
			split = 0
		case len(between) >= 2:
			pair := between[len(between)-2].symbol + between[len(between)-1].symbol
			if esOnsets[pair] {
				split = len(between) - 2
			} else {
				split = len(between) - 1
			}
		}
		syllables[n].coda = between[:split]
		syllables[n+1].onset = between[split:]
	}
	syllables[len(syllables)-1].coda = phones[nuclei[len(nuclei)-1].end:]
	return syllables
}

// esAssignStressは強勢を決める。綴りの強勢符号、なければ語末の文字で決め、長い語には第2強勢を置く。
func esAssignStress(word string, syllables []esSyllable) {
	if base, ok := strings.CutSuffix(word, "mente"); ok && len(syllables) > 2 && strings.IndexFunc(base, esIsVowelLetter) >= 0 {
		// 副詞の-menteは、元の語と-menteの両方に強勢がある。
		head := len(syllables) - 2
		esAssignWordStress(base, syllables[:head])
		syllables[head].stress = 1
		syllables[head].closedE = true
		return
	}
	esAssignWordStress(word, syllables)
}

func esAssignWordStress(word string, syllables []esSyllable) {
	primary := -1
	for i, syllable := range syllables {
		for _, phone := range syllable.nucleus {
			if phone.accent {
				primary = i
			}
		}
	}
	if primary < 0 {
		primary = len(syllables) - 1
		last := []rune(word)[len([]rune(word))-1]
		if len(syllables) > 1 && (strings.ContainsRune("aeiouns", last)) {
			primary = len(syllables) - 2
		}
	}
	syllables[primary].stress = 1
	for i := 0; i <= primary-2; i += 2 {
		syllables[i].stress = 2
	}
}

// esRenderは音節を読みにする。prevは前の語の最後の音で、閉鎖音か摩擦音かを決めるのに使う。
func esRender(syllables []esSyllable, prev string) string {
	type flat struct {
		phone    esPhone
		syllable int
		role     int // 0=onset 1=nucleus 2=coda
	}
	var all []flat
	for s, syllable := range syllables {
		for _, p := range syllable.onset {
			all = append(all, flat{p, s, 0})
		}
		for _, p := range syllable.nucleus {
			all = append(all, flat{p, s, 1})
		}
		for _, p := range syllable.coda {
			all = append(all, flat{p, s, 2})
		}
	}
	symbols := make([]string, len(all))
	for i, item := range all {
		symbols[i] = item.phone.symbol
	}
	before := func(i int) string {
		if i > 0 {
			return symbols[i-1]
		}
		return prev
	}
	after := func(i int) string {
		if i+1 < len(all) {
			return all[i+1].phone.symbol
		}
		return ""
	}
	for i, item := range all {
		p := item.phone
		switch {
		case p.symbol == "b" || p.symbol == "d" || p.symbol == "ɡ":
			b := before(i)
			stop := b == "" || b == "m" || b == "n" || b == "ŋ"
			// dは、語末とwの前で閉鎖音のまま（ciudad → θjuðˈad、duele → dwˈele）。
			// 語末のdが次の語の母音に続くときは、esAssimilateAcrossWordsでðにする。
			if p.symbol == "d" && (i == len(all)-1 || (i+1 < len(all) && all[i+1].phone.symbol == "u" && !all[i+1].phone.strong && i+2 < len(all) && all[i+2].phone.vowel)) {
				stop = true
			}
			if !stop {
				symbols[i] = map[string]string{"b": "β", "d": "ð", "ɡ": "ɣ"}[p.symbol]
			}
		case p.symbol == "n":
			switch after(i) {
			case "b", "p":
				symbols[i] = "m"
			case "ɡ", "x":
				symbols[i] = "ŋ"
			}
		case p.symbol == "ɾ":
			if p.roll {
				symbols[i] = "r"
			}
		}
	}
	var b strings.Builder
	for s, syllable := range syllables {
		if s > 0 {
			b.WriteString(".")
		}
		var onset, nucleus, coda []string
		for i, item := range all {
			if item.syllable != s {
				continue
			}
			switch item.role {
			case 0:
				onset = append(onset, symbols[i])
			case 1:
				nucleus = append(nucleus, symbols[i])
			default:
				coda = append(coda, symbols[i])
			}
		}
		core := esNucleus(syllable.nucleus, nucleus, syllable.stress == 1 && !syllable.closedE && esOpenE(syllable, s, syllables))
		mark := ""
		switch syllable.stress {
		case 1:
			mark = esPrimary
		case 2:
			mark = esSecondary
		}
		b.WriteString(strings.Join(onset, ""))
		b.WriteString(core.glides)
		b.WriteString(mark)
		b.WriteString(core.vowel)
		b.WriteString(strings.Join(coda, ""))
	}
	return b.String()
}

type esCore struct{ glides, vowel string }

// esNucleusは核の母音の並びを、前の半母音と、強勢記号を置く母音（後ろの半母音を含む）に分ける。
func esNucleus(phones []esPhone, symbols []string, openE bool) esCore {
	// 核の中心は、強い母音か、最後の母音（iu → ju、ui → wi）。
	center := len(phones) - 1
	for i, p := range phones {
		if p.strong {
			center = i
			break
		}
	}
	if !phones[center].strong && phones[len(phones)-1].fromY {
		center = 0
	}
	var glides strings.Builder
	for i := 0; i < center; i++ {
		glides.WriteString(map[string]string{"i": "j", "u": "w"}[symbols[i]])
	}
	vowel := symbols[center]
	tail := ""
	for i := center + 1; i < len(symbols); i++ {
		tail += symbols[i]
	}
	if vowel == "e" && tail == "" && openE {
		vowel = "ɛ"
	}
	switch vowel + tail {
	case "ai":
		return esCore{glides.String(), "I"}
	case "au":
		return esCore{glides.String(), "W"}
	case "ei":
		return esCore{glides.String(), "A"}
	case "eu":
		return esCore{glides.String(), "eʊ"}
	case "oi":
		return esCore{glides.String(), "oɪ"}
	case "ui":
		return esCore{glides.String(), "uj"}
	}
	for _, t := range tail {
		vowel += map[rune]string{'i': "j", 'u': "w"}[t]
	}
	return esCore{glides.String(), vowel}
}

// esOpenEは、強勢のあるeが、語中で音節末のnの前にあって広いɛになるか（gente → xˈɛnte）。
func esOpenE(syllable esSyllable, index int, syllables []esSyllable) bool {
	return len(syllable.coda) > 0 && syllable.coda[0].symbol == "n" && index+1 < len(syllables)
}
