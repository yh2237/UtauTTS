package frontend

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// ポルトガル語（ブラジル）の読みは、スペイン語と同じ形（音節を「.」で、語を空白でつなぐ）。
// 記号はmisakiがespeak-ng（pt-br）の出力を直したものに合わせる。強勢のe、oの開き方は語ごとに
// 決まるので、規則では近い方を選ぶ。

// ParsePortugueseは文か読みからポルトガル語の読みとモーラ（1音節を1モーラ）を返す。
func ParsePortuguese(text, reading string, dictionary map[string]string) (string, []Mora, error) {
	if strings.TrimSpace(reading) == "" {
		var err error
		reading, err = PortugueseReading(text, dictionary)
		if err != nil {
			return "", nil, err
		}
	}
	morae, err := syllableMorae(reading, LanguagePortuguese)
	if err != nil {
		return "", nil, err
	}
	return reading, morae, nil
}

// ptUnstressedは、2語以上の文で強勢を置かない機能語。
var ptUnstressed = map[string]bool{
	"o": true, "a": true, "os": true, "as": true, "de": true, "do": true, "da": true, "dos": true, "das": true,
	"no": true, "na": true, "nos": true, "nas": true, "um": true, "que": true, "e": true,
	"se": true, "me": true, "te": true, "lhe": true, "por": true, "com": true, "eu": true,
	"tem": true, "têm": true, "vai": true, "faz": true, "ser": true, "meu": true, "teu": true, "seu": true,
	"foi": true,
}

// ptWordsは規則に合わない語の読み（音節を「.」で区切る）。
var ptWords = map[string]string{
	"muito": "mwˈiŋ.tʊ", "muitos": "mwˈiŋ.tʊs", "muita": "mwˈiŋ.tæ", "muitas": "mwˈiŋ.tæs",
	"ruim": "xu.ˈiŋ", "quer": "kˈɛr", "mulher": "mu.ljˈɛr", "à": "ˌaː", "a": "a",
	"um": "ũŋ", "ou": "ow", "uma": "ˌu.mæ", "onde": "ˌoŋ.ʤy", "ao": "W", "aos": "Ws", "às": "aːs",
	"e": "i", "é": "ɛ", "há": "a",
}

// ptSecondaryWordsは、文の中で第2強勢になる語。
var ptSecondaryWords = map[string]bool{
	"para": true, "pra": true, "não": true, "você": true, "está": true, "minha": true, "minhas": true,
	"nunca": true, "tinha": true, "pode": true, "esta": true, "este": true, "estas": true, "estes": true,
	"essa": true, "esse": true, "ela": true, "ele": true, "elas": true, "eles": true, "era": true,
	"temos": true, "sempre": true, "desde": true, "vamos": true, "estou": true, "dela": true, "dele": true,
	"sua": true, "suas": true, "numa": true, "tenho": true, "neste": true, "nesta": true,
}

// PortugueseReadingは文をポルトガル語の読みにする。dictionaryの語（小文字）は、その読みを使う。
func PortugueseReading(text string, dictionary map[string]string) (string, error) {
	tokens := ptTokens(text)
	words := 0
	for _, token := range tokens {
		if token.word != "" {
			words++
		}
	}
	if words == 0 {
		return "", fmt.Errorf("no Portuguese words in %q", text)
	}
	var fields []string
	join := false
	for _, token := range tokens {
		if token.pause != "" {
			if len(fields) > 0 {
				fields = append(fields, token.pause)
			}
			join = false
			continue
		}
		text, err := ptWordReading(token.word, dictionary, words > 1)
		if err != nil {
			return "", err
		}
		if join && len(fields) > 0 {
			// ハイフンでつないだ語は1語として読む（arco-íris → ˈaɾ.kʊ.ˈi.ɾis）。
			fields[len(fields)-1] += "." + text
		} else {
			fields = append(fields, text)
		}
		join = token.joinNext
	}
	ptAcrossWords(fields)
	return strings.Join(fields, " "), nil
}

// ptWordReadingは1語の読みを返す。inSentenceなら機能語の強勢を弱める。
func ptWordReading(word string, dictionary map[string]string, inSentence bool) (string, error) {
	if value := dictionary[word]; value != "" {
		return value, nil
	}
	if value := ptWords[word]; value != "" {
		return value, nil
	}
	syllables, err := portugueseWord(word)
	if err != nil {
		return "", err
	}
	if inSentence {
		switch {
		case ptUnstressed[word]:
			for i := range syllables {
				syllables[i].stress = 0
			}
		case ptSecondaryWords[word]:
			for i := range syllables {
				if syllables[i].stress == 1 {
					syllables[i].stress = 2
					syllables[i].demoted = true
				}
			}
		}
	}
	return ptRender(syllables), nil
}

func ptTokens(text string) []syllableToken {
	text = norm.NFC.String(strings.ToLower(text))
	var tokens []syllableToken
	var word strings.Builder
	flush := func() {
		if word.Len() > 0 {
			tokens = append(tokens, syllableToken{word: word.String()})
			word.Reset()
		}
	}
	for _, r := range text {
		switch {
		case unicode.IsLetter(r):
			word.WriteRune(r)
		case r == '-' && word.Len() > 0:
			flush()
			tokens[len(tokens)-1].joinNext = true
		default:
			flush()
			if pause := syllablePauseSymbol(r); pause != "" {
				if len(tokens) > 0 && tokens[len(tokens)-1].pause != "" {
					continue
				}
				tokens = append(tokens, syllableToken{pause: pause})
			}
		}
	}
	flush()
	return tokens
}

// ptPhoneは語の中の1つの音。
type ptPhone struct {
	symbol string
	vowel  bool
	// accentは綴りの記号（'´'鋭、'^'曲、'~'鼻、0なし）。
	accent rune
	nasal  bool
	// glideは、前の母音と二重母音になるi、u。
	glide bool
	// nucleusは、そのまま核として書く記号（ão、ãe、õe、語末の-am、-em）。
	nucleus string
	// codaNasalは、鼻母音の後のm、n（m・ŋとして書く）。
	codaNasal bool
	// rは、rの種類（'x'強い、'r'子音の後、'ɾ'弱い、'c'音節末、'f'語末）。
	r rune
	// lCodaは、音節末のl（ʊになる）。
	lCoda bool
}

type ptSyllable struct {
	onset   []ptPhone
	nucleus []ptPhone
	coda    []ptPhone
	stress  int
	// demotedは、文の中で第1強勢を第2強勢に弱めた音節。母音の質は第1強勢のままにする。
	demoted bool
}

func ptIsVowelLetter(r rune) bool { return strings.ContainsRune("aeiouáàâãéêíóôõúü", r) }

func ptVowel(r rune) (string, rune) {
	switch r {
	case 'á', 'à':
		return "a", '´'
	case 'â':
		return "a", '^'
	case 'ã':
		return "a", '~'
	case 'é':
		return "e", '´'
	case 'ê':
		return "e", '^'
	case 'í':
		return "i", '´'
	case 'ó':
		return "o", '´'
	case 'ô':
		return "o", '^'
	case 'õ':
		return "o", '~'
	case 'ú':
		return "u", '´'
	case 'ü':
		return "u", 0
	}
	return string(r), 0
}

func ptLetters(word string) ([]ptPhone, error) {
	runes := []rune(word)
	at := func(i int) rune {
		if i < 0 || i >= len(runes) {
			return 0
		}
		return runes[i]
	}
	front := func(r rune) bool { return strings.ContainsRune("eéêiíy", r) }
	consonantAfter := func(i int) bool {
		r := at(i)
		return r != 0 && !ptIsVowelLetter(r) && r != 'h'
	}
	var phones []ptPhone
	add := func(symbol string) { phones = append(phones, ptPhone{symbol: symbol}) }
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if ptIsVowelLetter(r) {
			base, accent := ptVowel(r)
			next := at(i + 1)
			// 鼻音の二重母音
			if accent == '~' && (next == 'o' || next == 'e') {
				switch {
				case base == "a" && next == 'o':
					phones = append(phones, ptPhone{symbol: "a", vowel: true, accent: accent, nucleus: "ɐ̃ʊ̃"})
				case base == "a" && next == 'e':
					phones = append(phones, ptPhone{symbol: "a", vowel: true, accent: accent, nucleus: "ɐ̃y"})
				default:
					phones = append(phones, ptPhone{symbol: "o", vowel: true, accent: accent, nucleus: "õj"})
				}
				i++
				continue
			}
			phone := ptPhone{symbol: base, vowel: true, accent: accent, nasal: accent == '~'}
			// 前の母音に続く強勢のないi、uは二重母音の後半（leite、pouco）。
			if (base == "i" || base == "u") && accent == 0 && len(phones) > 0 && phones[len(phones)-1].vowel &&
				phones[len(phones)-1].nucleus == "" && !phones[len(phones)-1].glide {
				// 語末のrの前（sair、cair）は二重母音にしない。
				phone.glide = !(next == 'r' && at(i+2) == 0)
			}
			// 母音の後のm、nは、子音の前と語末で鼻母音にする。
			if (next == 'm' || next == 'n') && (at(i+2) == 0 || consonantAfter(i+2)) && at(i+2) != 'h' {
				phone.nasal = true
				if at(i+2) == 0 {
					switch base {
					case "a":
						if next == 'm' {
							phone.nucleus = "ɐ̃ʊ̃"
						}
					case "e":
						phone.nucleus = "Aŋ"
					}
				}
				phones = append(phones, phone)
				if phone.nucleus == "" {
					phones = append(phones, ptPhone{symbol: "n", codaNasal: true})
				}
				i++
				// 語末の-ens（jovens）は鼻母音のあとにsが続く。
				continue
			}
			phones = append(phones, phone)
			continue
		}
		switch r {
		case 'b':
			add("b")
		case 'c':
			switch next := at(i + 1); {
			case next == 'h':
				add("ʃ")
				i++
			case front(next):
				add("s")
			default:
				add("k")
			}
		case 'ç':
			add("s")
		case 'd':
			add("d")
		case 'f':
			add("f")
		case 'g':
			next := at(i + 1)
			switch {
			case front(next):
				add("ʒ")
			case next == 'u' && front(at(i+2)):
				add("ɡ")
				i++
			case next == 'u' && ptIsVowelLetter(at(i+2)):
				add("ɡ")
				phones = append(phones, ptPhone{symbol: "w"})
				i++
			default:
				add("ɡ")
			}
		case 'h':
		case 'j':
			add("ʒ")
		case 'k':
			add("k")
		case 'l':
			if at(i+1) == 'h' {
				add("l")
				add("j")
				i++
				continue
			}
			coda := at(i+1) == 0 || consonantAfter(i+1)
			phones = append(phones, ptPhone{symbol: "l", lCoda: coda})
		case 'm':
			add("m")
		case 'n':
			if at(i+1) == 'h' {
				add("ɲ")
				i++
			} else {
				add("n")
			}
		case 'p':
			add("p")
		case 'q':
			add("k")
			if at(i+1) == 'u' {
				if front(at(i + 2)) {
					i++
				} else if ptIsVowelLetter(at(i + 2)) {
					phones = append(phones, ptPhone{symbol: "w"})
					i++
				}
			}
		case 'r':
			kind := 'ɾ'
			previous := at(i - 1)
			switch {
			case i == 0 || at(i+1) == 'r' || strings.ContainsRune("lns", previous):
				kind = 'x'
				if at(i+1) == 'r' {
					i++
				}
			case previous != 0 && !ptIsVowelLetter(previous):
				kind = 'r'
			case at(i+1) == 0:
				kind = 'f'
			case consonantAfter(i + 1):
				kind = 'c'
			}
			phones = append(phones, ptPhone{symbol: "r", r: kind})
		case 's':
			if at(i+1) == 's' {
				add("s")
				i++
				continue
			}
			if at(i+1) == 'c' && front(at(i+2)) {
				add("s")
				i++
				continue
			}
			if i > 0 && ptIsVowelLetter(at(i-1)) && (ptIsVowelLetter(at(i+1)) || strings.ContainsRune("bdgvzjmnlr", at(i+1))) {
				add("z")
			} else {
				add("s")
			}
		case 't':
			add("t")
		case 'v':
			add("v")
		case 'w':
			add("w")
		case 'x':
			previous := at(i - 1)
			switch {
			case i == 0:
				add("ʃ")
			case i == 1 && previous == 'e' && ptIsVowelLetter(at(i+1)):
				add("z")
			case i == 1 && previous == 'e':
				add("s")
			case strings.ContainsRune("iu", previous) && i >= 2 && ptIsVowelLetter(at(i-2)):
				add("ʃ")
			case i >= 1 && ptIsVowelLetter(previous) && at(i+1) == 0:
				add("k")
				add("s")
			default:
				add("k")
				add("s")
			}
		case 'y':
			phones = append(phones, ptPhone{symbol: "i", vowel: true})
		case 'z':
			if at(i+1) == 0 {
				add("s")
			} else {
				add("z")
			}
		default:
			return nil, fmt.Errorf("unsupported letter %q in Portuguese word %q", string(r), word)
		}
	}
	return phones, nil
}

var ptOnsets = map[string]bool{
	"pr": true, "br": true, "tr": true, "dr": true, "kr": true, "ɡr": true, "fr": true, "vr": true,
	"pl": true, "bl": true, "kl": true, "ɡl": true, "fl": true, "lj": true, "kw": true, "ɡw": true,
}

func ptSyllabify(phones []ptPhone) []ptSyllable {
	type group struct{ start, end int }
	var nuclei []group
	for i := 0; i < len(phones); {
		if !phones[i].vowel {
			i++
			continue
		}
		end := i + 1
		for end < len(phones) && phones[end].vowel && phones[end].glide {
			end++
		}
		nuclei = append(nuclei, group{i, end})
		i = end
	}
	if len(nuclei) == 0 {
		return nil
	}
	syllables := make([]ptSyllable, len(nuclei))
	for n, g := range nuclei {
		syllables[n].nucleus = phones[g.start:g.end]
	}
	syllables[0].onset = phones[:nuclei[0].start]
	for n := 0; n+1 < len(nuclei); n++ {
		between := phones[nuclei[n].end:nuclei[n+1].start]
		split := 0
		if len(between) >= 2 {
			pair := between[len(between)-2].symbol + between[len(between)-1].symbol
			if ptOnsets[pair] {
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

func portugueseWord(word string) ([]ptSyllable, error) {
	phones, err := ptLetters(word)
	if err != nil {
		return nil, err
	}
	syllables := ptSyllabify(phones)
	if len(syllables) == 0 {
		return nil, fmt.Errorf("Portuguese word %q has no vowel", word)
	}
	ptAssignStress(word, syllables)
	return ptMergePostTonicI(syllables), nil
}

// ptMergePostTonicIは、強勢の後のiが次の母音と続くとき、半母音jとして次の音節にまとめる（infância → ˈɐ̃ŋ.sjæ）。
func ptMergePostTonicI(syllables []ptSyllable) []ptSyllable {
	primary := -1
	for i, syllable := range syllables {
		if syllable.stress == 1 {
			primary = i
		}
	}
	for i := len(syllables) - 2; i > primary && i >= 0; i-- {
		syllable, next := syllables[i], syllables[i+1]
		if len(syllable.nucleus) != 1 || syllable.nucleus[0].symbol != "i" || len(syllable.coda) > 0 || len(next.onset) > 0 || next.nucleus[0].nucleus != "" {
			continue
		}
		merged := ptSyllable{stress: next.stress, coda: next.coda,
			onset:   append(append([]ptPhone(nil), syllable.onset...), ptPhone{symbol: "j"}),
			nucleus: next.nucleus}
		syllables = append(syllables[:i], append([]ptSyllable{merged}, syllables[i+2:]...)...)
	}
	return syllables
}

// ptAssignStressは強勢を決める。綴りの記号、なければ語末の文字で決め、3音節以上の語の頭に第2強勢を置く。
func ptAssignStress(word string, syllables []ptSyllable) {
	primary := -1
	tilde := -1
	for i, syllable := range syllables {
		for _, phone := range syllable.nucleus {
			switch phone.accent {
			case '´', '^':
				primary = i
			case '~':
				tilde = i
			}
		}
	}
	if primary < 0 {
		primary = tilde
	}
	if primary < 0 {
		primary = len(syllables) - 1
		if len(syllables) > 1 && ptPenultimateStress(word) {
			primary = len(syllables) - 2
		}
	}
	syllables[primary].stress = 1
	if len(syllables) >= 3 && primary >= 1 {
		for i := 0; i <= primary-2 || i == 0; i += 2 {
			syllables[i].stress = 2
		}
	}
}

// ptPenultimateStressは、強勢の記号のない語が後ろから2番目の音節に強勢を持つかを返す。
func ptPenultimateStress(word string) bool {
	for _, ending := range []string{"a", "e", "o", "as", "es", "os", "am", "em", "ens"} {
		if strings.HasSuffix(word, ending) {
			// 二重母音で終わる語（-ei、-ou、-ai）と-ui は最後の音節。
			return !strings.HasSuffix(word, "ue") || strings.HasSuffix(word, "que") || strings.HasSuffix(word, "gue")
		}
	}
	return false
}

// ptRenderは音節を読みにする。
func ptRender(syllables []ptSyllable) string {
	primary := -1
	for i, syllable := range syllables {
		if syllable.stress == 1 {
			primary = i
		}
	}
	last := len(syllables) - 1
	var b strings.Builder
	for s, syllable := range syllables {
		if s > 0 {
			b.WriteString(".")
		}
		vowel, glides := ptNucleus(syllable, s, primary, last, syllables)
		for i, phone := range syllable.onset {
			b.WriteString(ptConsonant(phone, syllable.onset, i, vowel))
		}
		b.WriteString(glides)
		switch syllable.stress {
		case 1:
			b.WriteString(esPrimary)
		case 2:
			b.WriteString(esSecondary)
		}
		b.WriteString(vowel)
		for i, phone := range syllable.coda {
			next := ""
			if i+1 < len(syllable.coda) {
				next = syllable.coda[i+1].symbol
			} else if s+1 < len(syllables) && len(syllables[s+1].onset) > 0 {
				next = syllables[s+1].onset[0].symbol
			}
			b.WriteString(ptCoda(phone, next, vowel, s == last))
		}
	}
	text := b.String()
	// 前の音節の音節末のlが、母音とまとまったもの（mal → mˈW、azul → azˈuw）。
	return strings.NewReplacer("aʊ", "W", "uʊ", "uw", "oʊ", "ow").Replace(text)
}

// ptNucleusは、音節の核の母音を書く。
func ptNucleus(syllable ptSyllable, index, primary, last int, syllables []ptSyllable) (string, string) {
	head := syllable.nucleus[0]
	if head.nucleus != "" {
		if head.nucleus == "ɐ̃ʊ̃" && syllable.stress == 0 && index == last {
			return "ɐ̃ʊ̃", ""
		}
		return head.nucleus, ""
	}
	// 第2強勢の音節は弱めない（語末のa、e、oにしない）が、開き方と鼻音化は第1強勢だけ。
	stressed := syllable.stress == 1 || syllable.demoted
	reducible := syllable.stress == 0
	nextOnset := ""
	if index+1 < len(syllables) && len(syllables[index+1].onset) > 0 {
		nextOnset = syllables[index+1].onset[0].symbol
	}
	final := index == last && reducible && len(syllable.nucleus) == 1 &&
		(len(syllable.coda) == 0 || (len(syllable.coda) == 1 && syllable.coda[0].symbol == "s"))
	postTonic := primary >= 0 && index > primary
	var v string
	switch head.symbol {
	case "a":
		switch {
		case head.nasal:
			v = "ɐ̃"
		case len(syllable.coda) == 0 && nextOnset == "ɲ":
			v = "ɐ̃"
		case stressed && len(syllable.coda) == 0 && (nextOnset == "m" || nextOnset == "n"):
			v = "ɐ̃"
		case stressed:
			v = "a"
		case len(syllable.coda) == 0 && (nextOnset == "m" || nextOnset == "n"):
			v = "æ"
		case !reducible || len(syllables) == 1:
			v = "a"
		case final || postTonic:
			v = "æ"
		default:
			v = "a"
		}
	case "e":
		switch {
		case head.nasal:
			v = "A"
		case head.accent == '´':
			v = "ɛ"
		case head.accent == '^':
			v = "e"
		case stressed:
			v = ptStressedE(syllable, index, last, syllables)
		case final:
			v = "y"
		default:
			v = "e"
		}
	case "o":
		switch {
		case head.nasal:
			if stressed || len(syllable.coda) > 0 {
				v = "o"
			} else {
				v = "õ"
			}
		case head.accent == '´':
			v = "ɔ"
		case head.accent == '^':
			v = "o"
		case stressed:
			v = ptStressedO(syllable, index, last, syllables)
		case final:
			v = "ʊ"
		default:
			v = "o"
		}
	case "u":
		if head.nasal || (stressed && len(syllable.coda) == 0 && (nextOnset == "m" || nextOnset == "n" || nextOnset == "ɲ")) {
			v = "ũ"
		} else {
			v = "u"
		}
	default:
		v = head.symbol
	}
	for _, glide := range syllable.nucleus[1:] {
		v = ptDiphthong(v, glide.symbol)
	}
	return v, ""
}

func ptDiphthong(v, glide string) string {
	switch v + glide {
	case "ai":
		return "I"
	case "ei":
		return "A"
	case "ɛi":
		return "ɛɪ"
	case "oi":
		return "oɪ"
	case "ɔi":
		return "Y"
	case "au", "æu":
		return "W"
	case "eu":
		return "eʊ"
	case "ɛu":
		return "ɛʊ"
	case "ou", "ʊu":
		return "ow"
	case "ui":
		return "uj"
	case "iu":
		return "iʊ"
	}
	return v + map[string]string{"i": "j", "u": "w"}[glide]
}

// ptStressedEは記号のない強勢のeの開き方を選ぶ。不定詞の-erは狭いe、それ以外は広いɛ。
func ptStressedE(syllable ptSyllable, index, last int, syllables []ptSyllable) string {
	if index == last && len(syllable.coda) > 0 && syllable.coda[0].symbol == "r" {
		return "e"
	}
	if ptBeforeVowelOrNasal(syllable, index, syllables) {
		return "e"
	}
	if len(syllable.nucleus) > 1 {
		return "e"
	}
	return "ɛ"
}

// ptStressedOは記号のない強勢のoの開き方を選ぶ。語末の-o、-orは狭いo、-a、-e、-os、-ol、子音の前は広いɔ。
func ptStressedO(syllable ptSyllable, index, last int, syllables []ptSyllable) string {
	if len(syllable.nucleus) > 1 || ptBeforeVowelOrNasal(syllable, index, syllables) {
		return "o"
	}
	if index == last {
		if len(syllable.coda) > 0 && syllable.coda[0].symbol == "l" {
			return "ɔ"
		}
		return "o"
	}
	if index == last-1 {
		final := syllables[last]
		ending := final.nucleus[0].symbol
		plural := len(final.coda) > 0 && final.coda[len(final.coda)-1].symbol == "s"
		if ending == "o" && !plural && len(syllable.coda) == 0 {
			return "o"
		}
	}
	return "ɔ"
}

// ptBeforeVowelOrNasalは、音節末の子音がなく、次の音節が母音かm、n、ɲで始まるかを返す。
func ptBeforeVowelOrNasal(syllable ptSyllable, index int, syllables []ptSyllable) bool {
	if len(syllable.coda) > 0 || index+1 >= len(syllables) {
		return false
	}
	onset := syllables[index+1].onset
	return len(onset) == 0 || onset[0].symbol == "m" || onset[0].symbol == "n" || onset[0].symbol == "ɲ"
}

// ptConsonantは音節の頭の子音を書く。tとdはiの前でʧとʤになる。
func ptConsonant(phone ptPhone, onset []ptPhone, i int, vowel string) string {
	nextIsI := i == len(onset)-1 && (strings.HasPrefix(vowel, "i") || strings.HasPrefix(vowel, "y") || strings.HasPrefix(vowel, "ĩ"))
	switch phone.symbol {
	case "t":
		if nextIsI {
			return "ʧ"
		}
	case "d":
		if nextIsI {
			return "ʤ"
		}
	case "r":
		switch phone.r {
		case 'x':
			return "x"
		case 'r':
			return "r"
		}
		return "ɾ"
	}
	return phone.symbol
}

// ptCodaは音節末の子音を書く。
func ptCoda(phone ptPhone, next, vowel string, lastSyllable bool) string {
	switch {
	case phone.codaNasal:
		if next == "p" || next == "b" {
			return "m"
		}
		return "ŋ"
	case phone.symbol == "l" && phone.lCoda:
		if lastSyllable && next == "" && (strings.HasSuffix(vowel, "ɔ") || strings.HasSuffix(vowel, "o")) {
			return "l"
		}
		return "ʊ"
	case phone.symbol == "r":
		if phone.r == 'f' || (lastSyllable && next == "") {
			return "r"
		}
		if phone.r == 'x' {
			return "x"
		}
		return "ɾə"
	}
	return phone.symbol
}

// ptAcrossWordsは語の境目の音を直す。語末のsは有声音の前でz、語末のrは母音の前でɾ、語末のʊは母音の前でw。
func ptAcrossWords(fields []string) {
	for i := 0; i+1 < len(fields); i++ {
		if syllablePauses[fields[i+1]] != "" {
			continue
		}
		next := []rune(strings.NewReplacer(esPrimary, "", esSecondary, "").Replace(fields[i+1]))
		if len(next) == 0 {
			continue
		}
		vowelNext := strings.ContainsRune("aeiouɐɛɔæAIWYOʊyũõĩ", next[0])
		voicedNext := vowelNext || strings.ContainsRune("bdɡvzʒʤmnɲlrɾ", next[0])
		switch {
		case strings.HasSuffix(fields[i], "s") && voicedNext:
			fields[i] = strings.TrimSuffix(fields[i], "s") + "z"
		case strings.HasSuffix(fields[i], "r") && vowelNext:
			fields[i] = strings.TrimSuffix(fields[i], "r") + "ɾ"
		case strings.HasSuffix(fields[i], "ʊ") && vowelNext && strings.ContainsAny(fields[i], esPrimary+esSecondary) &&
			!strings.ContainsAny(string([]rune(fields[i])[len([]rune(fields[i]))-2]), "aeiouɛɔA"):
			fields[i] = strings.TrimSuffix(fields[i], "ʊ") + "w"
		case fields[i] == "ʊ" && vowelNext:
			fields[i] = "u"
		case strings.HasSuffix(fields[i], "y") && vowelNext:
			fields[i] = strings.TrimSuffix(fields[i], "y") + "j"
		}
	}
}
