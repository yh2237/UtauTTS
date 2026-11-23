package phonemizer

import (
	"fmt"
	"strings"

	"github.com/yh2237/UtauTTS/internal/linguistics"
	"github.com/yh2237/UtauTTS/internal/voicebank"
)

func init() {
	// 最初に登録したen-arpasingを英語の既定とする。
	Register(englishARPAsingPhonemizer{})
	Register(englishSyllablePhonemizer{id: linguistics.PhonemizerEnglishDelta, symbols: deltaEnglishSymbols, spacedStart: true})
	Register(englishSyllablePhonemizer{id: linguistics.PhonemizerEnglishVCCV, symbols: vccvEnglishSymbols, spacedStart: false})
	Register(englishCVPhonemizer{})
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

type englishARPAsingPhonemizer struct{}

func (englishARPAsingPhonemizer) ID() string       { return linguistics.PhonemizerEnglish }
func (englishARPAsingPhonemizer) Language() string { return linguistics.LanguageEnglish }

func (englishARPAsingPhonemizer) Aliases(utterance linguistics.Utterance, _ voicebank.PresampConfig) ([]voicebank.AliasHints, error) {
	hints := make([]voicebank.AliasHints, len(utterance.Morae))
	previous := "-"
	lastNonPause := -1
	for i, mora := range utterance.Morae {
		if mora.Pause {
			if lastNonPause >= 0 && i > 0 && !utterance.Morae[i-1].Pause {
				last := utterance.Morae[lastNonPause].Text
				hints[lastNonPause].Endings = [][]string{{last + " -", last + "-"}}
			}
			previous = "-"
			continue
		}
		symbol := mora.Text
		candidates := []string{previous + " " + symbol, symbol}
		if previous != "-" {
			candidates = append(candidates, "- "+symbol)
		}
		hints[i] = voicebank.AliasHints{Main: uniqueStrings(candidates)}
		previous = symbol
		lastNonPause = i
	}
	if len(utterance.Morae) > 0 && !utterance.Morae[len(utterance.Morae)-1].Pause && lastNonPause >= 0 {
		last := utterance.Morae[lastNonPause].Text
		hints[lastNonPause].Endings = [][]string{{last + " -", last + "-"}}
	}
	return hints, nil
}

type englishCVPhonemizer struct{}

func (englishCVPhonemizer) ID() string       { return linguistics.PhonemizerEnglishCV }
func (englishCVPhonemizer) Language() string { return linguistics.LanguageEnglish }

// 1音素を1 Moraとし、候補順はOpenUtauのEnglish C+V Phonemizer（Cadlaxa, MIT）に準拠する。
func (englishCVPhonemizer) Aliases(utterance linguistics.Utterance, _ voicebank.PresampConfig) ([]voicebank.AliasHints, error) {
	hints := make([]voicebank.AliasHints, len(utterance.Morae))
	phraseStart := true
	lastNonPause := -1
	for i, mora := range utterance.Morae {
		if mora.Pause {
			if lastNonPause >= 0 && i > 0 && !utterance.Morae[i-1].Pause {
				last := utterance.Morae[lastNonPause].Text
				hints[lastNonPause].Endings = [][]string{uniqueStrings([]string{last + " -", last + "-"})}
			}
			phraseStart = true
			continue
		}
		symbol := mora.Text
		var candidates []string
		if mora.Vowel != "" {
			candidates = cvVowelAliases(symbol, phraseStart)
		} else {
			candidates = cvConsonantAliases(symbol, phraseStart)
		}
		hints[i] = voicebank.AliasHints{
			Main:      candidates,
			MainKinds: repeatAliasKind("cv", len(candidates)),
		}
		phraseStart = false
		lastNonPause = i
	}
	if len(utterance.Morae) > 0 && !utterance.Morae[len(utterance.Morae)-1].Pause && lastNonPause >= 0 {
		last := utterance.Morae[lastNonPause].Text
		hints[lastNonPause].Endings = [][]string{uniqueStrings([]string{last + " -", last + "-"})}
	}
	return hints, nil
}

func cvVowelAliases(vowel string, phraseStart bool) []string {
	if phraseStart {
		return uniqueStrings([]string{"-" + vowel, "- " + vowel, vowel})
	}
	return uniqueStrings([]string{"-" + vowel, vowel, "- " + vowel})
}

func cvConsonantAliases(consonant string, phraseStart bool) []string {
	if phraseStart {
		return uniqueStrings([]string{"- " + consonant, "-" + consonant, consonant})
	}
	return uniqueStrings([]string{consonant, "-" + consonant, "- " + consonant})
}

type englishSyllablePhonemizer struct {
	id          string
	symbols     map[string][]string
	spacedStart bool
}

func (p englishSyllablePhonemizer) ID() string       { return p.id }
func (p englishSyllablePhonemizer) Language() string { return linguistics.LanguageEnglish }

func (p englishSyllablePhonemizer) Aliases(utterance linguistics.Utterance, presamp voicebank.PresampConfig) ([]voicebank.AliasHints, error) {
	morae := utterance.Morae
	hints := make([]voicebank.AliasHints, len(morae))
	const separator = " "
	var previousVowels []string
	var prevCoda []string
	phraseStart := true

	for i, mora := range morae {
		if mora.Pause {
			previousVowels, prevCoda, phraseStart = nil, nil, true
			continue
		}
		for _, phone := range mora.Phones {
			if len(p.symbols[phone.Symbol]) == 0 {
				return nil, fmt.Errorf("unsupported ARPAbet phoneme %q", phone.Symbol)
			}
		}
		onset, vowel, coda := syllablePhonesSplit(mora)
		vowels := englishSyllableVowels(vowel, mora.StressKnown, mora.Stress, p.symbols)
		if len(vowels) == 0 {
			return nil, fmt.Errorf("unsupported ARPAbet phoneme %q", vowel)
		}

		atPhraseStart := phraseStart
		mainOnset := onset
		if !atPhraseStart && len(mainOnset) == 0 && len(prevCoda) > 0 {
			mainOnset = prevCoda[len(prevCoda)-1:]
		}
		main := englishMainAliases(mainOnset, vowels, p.symbols)
		if !sameStrings(mainOnset, onset) {
			main = append(main, englishMainAliases(onset, vowels, p.symbols)...)
		}
		if atPhraseStart {
			prefix := "-"
			if p.spacedStart {
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
		if len(prevCoda) == 0 {
			transitions = combineEnglishTransitionAliases(previousVowels, onset, vowels, p.symbols, separator)
		}
		if len(onset) > 1 {
			transitions = append(transitions, englishOnsetClusterAliases(onset, p.symbols, separator, atPhraseStart)...)
		}
		main = uniqueStrings(main)
		hint := voicebank.AliasHints{
			Main:       main,
			MainKinds:  repeatAliasKind("cv", len(main)),
			Transition: uniqueStrings(transitions),
		}
		if atPhraseStart && len(onset) > 1 {
			hint.MainMissing = make(map[string][]string)
			for start := 1; start < len(onset); start++ {
				for _, alias := range combineEnglishAliases(onset[start:], vowels, p.symbols) {
					for _, prefix := range []string{"", "-", "- "} {
						hint.MainMissing[prefix+alias] = append([]string(nil), onset[:start]...)
					}
				}
			}
		}

		phraseEnd := i+1 == len(morae) || morae[i+1].Pause
		if len(coda) > 0 {
			if mora.WordEnd && phraseEnd {
				hint.Endings = englishTerminalConsonants(vowels, coda, p.symbols, separator)
			} else {
				var nextOnset []string
				if !phraseEnd {
					nextOnset, _, _ = syllablePhonesSplit(morae[i+1])
				}
				hint.Endings = englishSyllableBridge(vowels, coda, nextOnset, p.symbols, separator)
			}
			hint.EndingPhones = [][]string{append([]string(nil), coda[:1]...)}
			if len(hint.Endings) > 1 {
				hint.EndingPhones = append(hint.EndingPhones, append([]string(nil), coda[1:]...))
				if len(coda) > 2 {
					hint.EndingFallbacks = make([][]voicebank.CodaAlias, len(hint.Endings))
					hint.EndingFallbacks[1] = englishCodaChain(coda, p.symbols, separator, mora.WordEnd && phraseEnd)
				}
			}
			if !p.spacedStart {
				// VCCVのIH+NGは専用の韻名を使う。
				if vowel == "ih" && coda[0] == "ng" {
					hint.Endings[0] = append([]string{"1 ng", "1ng"}, hint.Endings[0]...)
				}
				if sameStrings(coda, []string{"s", "p"}) {
					var cluster []string
					for _, v := range vowels {
						cluster = append(cluster, v+" sp", v+"sp")
					}
					// VC連続録音と分割経路を比較する。
					first := hint.Endings[0]
					second := hint.Endings[1]
					hint.Endings = [][]string{cluster}
					hint.EndingPhones = [][]string{{"s", "p"}}
					hint.EndingFallbacks = [][]voicebank.CodaAlias{{
						{Aliases: cluster, Phones: []string{"s", "p"}, CodaStart: 0},
						{Aliases: first, Phones: []string{"s"}, CodaStart: 0},
						{Aliases: second, Phones: []string{"p"}, CodaStart: 1},
					}}
				}
			}
		} else if mora.WordEnd && phraseEnd {
			hint.Endings = [][]string{englishEndingAliases(vowels, presamp)}
		}

		hints[i] = hint
		if len(coda) > 0 {
			previousVowels = nil
		} else {
			previousVowels = vowels
		}
		prevCoda = coda
	}
	return hints, nil
}

func syllablePhonesSplit(mora linguistics.Mora) (onset []string, vowel string, coda []string) {
	for _, phone := range mora.Phones {
		switch phone.Role {
		case "onset":
			onset = append(onset, phone.Symbol)
		case "coda":
			coda = append(coda, phone.Symbol)
		case "nucleus":
			if vowel == "" {
				vowel = phone.Symbol
			}
		}
	}
	if vowel == "" {
		vowel = mora.Vowel
	}
	return onset, vowel, coda
}

// CMUdictのAH0は曖昧母音として扱い AXエイリアスを優先する。
func englishSyllableVowels(vowel string, stressKnown bool, stress int, symbols map[string][]string) []string {
	if vowel == "ah" && stressKnown && stress == 0 {
		return uniqueStrings(append(append([]string(nil), symbols["ax"]...), symbols["ah"]...))
	}
	return symbols[vowel]
}

func englishCodaChain(coda []string, symbols map[string][]string, separator string, terminal bool) []voicebank.CodaAlias {
	var result []voicebank.CodaAlias
	for i := 1; i < len(coda); i++ {
		for end := i + 1; end <= len(coda); end++ {
			var aliases []string
			leftSymbols := symbols[coda[i-1]]
			if coda[i-1] == "ng" && (coda[i] == "k" || coda[i] == "g") && symbols["ng"][0] == "N" {
				// Delta音源の軟口蓋音前の鼻音表記を補う。
				leftSymbols = append(append([]string(nil), leftSymbols...), "n")
			}
			for _, left := range leftSymbols {
				for _, right := range combineEnglishAliases(coda[i:end], []string{""}, symbols) {
					pair := []string{left + separator + right, left + right}
					if terminal && end == len(coda) {
						for _, name := range pair {
							aliases = append(aliases, name+"-")
						}
					}
					aliases = append(aliases, pair...)
					if !terminal || end != len(coda) {
						for _, name := range pair {
							aliases = append(aliases, name+"-")
						}
					}
				}
			}
			result = append(result, voicebank.CodaAlias{
				Aliases:   uniqueStrings(aliases),
				Phones:    append([]string(nil), coda[i:end]...),
				CodaStart: i,
			})
		}
	}
	return result
}

func englishEndingAliases(vowels []string, config voicebank.PresampConfig) []string {
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
		// 解放マーカーが欠けても利用可能なVCを候補から消さない。
		for _, vowel := range vowels {
			for _, value := range cluster {
				endings = append(endings, vowel+separator+value, vowel+value)
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
			restAliases = append(restAliases,
				left+separator+right+"-", left+right+"-",
				left+separator+right, left+right)
		}
	}
	for _, value := range rest {
		restAliases = append(restAliases, value+"-", value)
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
			// 語境界のCCは解放マーカー付きを優先し、非マーカー形は後段に残す。
			bridge = append(bridge, left+separator+right+"-", left+right+"-", left+separator+right, left+right)
		}
	}
	// 語境界の子音連続がなくても語末子音全体を選べるようにする。
	if len(coda) > 1 && len(nextOnset) > 0 {
		codaRest := combineEnglishAliases(coda[1:], []string{""}, symbols)
		for _, left := range first {
			for _, right := range codaRest {
				bridge = append(bridge, left+separator+right, left+right, left+separator+right+"-", left+right+"-")
			}
		}
	}
	return append(result, uniqueStrings(bridge))
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

// 母音始まりはVCでなくVV候補にする。
func combineEnglishTransitionAliases(previous []string, onset []string, vowels []string, symbols map[string][]string, separator string) []string {
	if len(previous) == 0 {
		return nil
	}
	if len(onset) == 0 {
		return combineEnglishVowelTransitionAliases(previous, vowels, separator)
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

func combineEnglishVowelTransitionAliases(previous, vowels []string, separator string) []string {
	var result []string
	for _, left := range previous {
		for _, right := range vowels {
			result = append(result,
				left+separator+right+"-",
				left+right+"-",
				left+separator+right,
				left+right,
			)
		}
	}
	return uniqueStrings(result)
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
