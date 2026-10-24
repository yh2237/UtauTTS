//go:build !js

package tts

import (
	"strings"

	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/refspeech"
)

// englishPhoneSymbolsは、ARPAbetの音素（小文字、強勢なし）ごとに、Kokoroの記号の候補。
var englishPhoneSymbols = map[string]string{
	"aa": "ɑɒ", "ae": "æ", "ah": "əʌ", "ao": "ɔɑ", "aw": "W", "ay": "I", "eh": "ɛe", "er": "ɜɚəɹ",
	"ey": "A", "ih": "ɪᵻi", "iy": "i", "ow": "Oo", "oy": "Y", "uh": "ʊ", "uw": "u",
	"b": "b", "ch": "ʧ", "d": "dɾ", "dh": "ð", "f": "f", "g": "ɡ", "hh": "h", "jh": "ʤ", "k": "k",
	"l": "lɫ", "m": "m", "n": "nɾ", "ng": "ŋ", "p": "p", "r": "ɹ", "s": "s", "sh": "ʃ", "t": "tɾʔ",
	"th": "θ", "v": "v", "w": "w", "y": "j", "z": "z", "zh": "ʒ",
}

const kokoroVowelSymbols = "ɑɒæəʌɔWIɛeɜɚAɪᵻiOoYʊua"

// alignEnglishWordは、語のKokoroの記号を語の音素へ順に割り当て、記号ごとの音素の番号を返す。
// 記号が音素の候補なら0、強勢の記号は後ろの母音に、長さなどの修飾の記号は隣に、低いコストで付ける。
// 記号のない音素には1を足す。
func alignEnglishWord(symbols []string, phones []frontend.Phone) []int {
	n, m := len(symbols), len(phones)
	if n == 0 || m == 0 {
		return nil
	}
	vowelPhone := func(p int) bool {
		return strings.ContainsAny(englishPhoneSymbols[phones[p].Symbol], kokoroVowelSymbols)
	}
	cost := func(i, p int) float64 {
		s := symbols[i]
		switch {
		case strings.Contains(englishPhoneSymbols[phones[p].Symbol], s) && s != "":
			return 0
		case s == "ˈ" || s == "ˌ":
			if vowelPhone(p) {
				return 0
			}
			return 0.5
		case s == "ː" || s == "ᵊ" || s == "ʰ":
			return 0.1
		case strings.Contains(kokoroVowelSymbols, s) == vowelPhone(p):
			return 0.6
		}
		return 1
	}
	const skip = 1.0
	inf := 1e18
	dp := make([][]float64, n)
	from := make([][]int, n)
	for i := range dp {
		dp[i] = make([]float64, m)
		from[i] = make([]int, m)
		for p := range dp[i] {
			dp[i][p] = inf
		}
	}
	for p := range m {
		dp[0][p] = float64(p)*skip + cost(0, p)
	}
	for i := 1; i < n; i++ {
		for p := range m {
			if dp[i-1][p] >= inf {
				continue
			}
			for q := p; q < m; q++ {
				gap := 0.0
				if q > p {
					gap = float64(q-p-1) * skip
				}
				if v := dp[i-1][p] + gap + cost(i, q); v < dp[i][q] {
					dp[i][q], from[i][q] = v, p
				}
			}
		}
	}
	best, end := inf, 0
	for p := range m {
		if v := dp[n-1][p] + float64(m-1-p)*skip; v < best {
			best, end = v, p
		}
	}
	assigned := make([]int, n)
	for i, p := n-1, end; i >= 0; i-- {
		assigned[i] = p
		p = from[i][p]
	}
	return assigned
}

// englishWordAssignmentは、語の記号を音素の並びで語のモーラへ割り当てる。音素が分からなければnilを返す。
func englishWordAssignment(pieces []refspeech.Piece, timing refspeech.Timing, tokens []int, morae []frontend.Mora) [][]int {
	var phones []frontend.Phone
	var moraOf []int
	for i, mora := range morae {
		for _, phone := range mora.Phones {
			phones = append(phones, phone)
			moraOf = append(moraOf, i)
		}
	}
	if len(phones) == 0 || len(tokens) == 0 {
		return nil
	}
	symbols := make([]string, len(tokens))
	for i, token := range tokens {
		symbols[i] = tokenSymbol(timing, pieces, token)
	}
	assigned := alignEnglishWord(symbols, phones)
	if assigned == nil {
		return nil
	}
	result := make([][]int, len(morae))
	for i, token := range tokens {
		mora := moraOf[assigned[i]]
		result[mora] = append(result[mora], token)
	}
	return result
}

// tokenSymbolは、Kokoroの入力記号の番号から記号を返す。断片の外（境界記号）なら空文字。
func tokenSymbol(timing refspeech.Timing, pieces []refspeech.Piece, token int) string {
	for i, span := range timing.PieceTokens {
		if token >= span[0] && token < span[1] {
			if runes := []rune(pieces[i].Symbols); token-span[0] < len(runes) {
				return string(runes[token-span[0]])
			}
		}
	}
	return ""
}
