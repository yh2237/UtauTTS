package refspeech

import (
	"math"
	"strings"

	"github.com/yh2237/UtauTTS/internal/linguistics"
)

// simpleMoraTokensは、断片とモーラが1対1で並ぶ言語（中国語など）で、
// Kokoroの入力記号をモーラへ割り当てる。先頭の境界記号はどのモーラにも入れず、
// 末尾の境界記号は最後のモーラに入れる。
func simpleMoraTokens(pieces []Piece, timing Timing, morae []linguistics.Mora) [][]int {
	result := make([][]int, len(morae))
	if len(pieces) != len(morae) || len(timing.PieceTokens) != len(pieces) {
		return result
	}
	for i := range morae {
		for token := timing.PieceTokens[i][0]; token < timing.PieceTokens[i][1]; token++ {
			result[i] = append(result[i], token)
		}
	}
	if len(timing.Durations) > 0 && len(result) > 0 {
		result[len(result)-1] = append(result[len(result)-1], len(timing.Durations)-1)
	}
	return result
}

// partMoraTokensは、断片のPartがモーラの番号の言語（スペイン語など）で、Kokoroの入力記号をモーラへ割り当てる。
// 語の間の空白や文末の記号（Partが-1）は前のモーラに入れ、末尾の境界記号は最後のモーラに入れる。
func partMoraTokens(pieces []Piece, timing Timing, morae []linguistics.Mora) [][]int {
	result := make([][]int, len(morae))
	if len(timing.PieceTokens) != len(pieces) || len(morae) == 0 {
		return result
	}
	current := -1
	for i, piece := range pieces {
		if piece.Part >= 0 && piece.Part < len(morae) {
			current = piece.Part
		}
		if current < 0 {
			continue
		}
		for token := timing.PieceTokens[i][0]; token < timing.PieceTokens[i][1]; token++ {
			result[current] = append(result[current], token)
		}
	}
	if len(timing.Durations) > 0 {
		result[len(result)-1] = append(result[len(result)-1], len(timing.Durations)-1)
	}
	return result
}

// englishMoraTokensは、Kokoroの入力記号をモーラへ割り当てる。
// 語の記号は、語の中のモーラへ音素の数の比で順に分け、語の後ろの空白は語の最後のモーラに入れる。
// 休止のモーラには読点を、文末の休止には文末の記号と末尾の境界記号も入れる。
// 文末に休止がなければ、それらは最後のモーラに入れる。先頭の境界記号はどのモーラにも入れない。
func englishMoraTokens(pieces []Piece, timing Timing, reading string, morae []linguistics.Mora) [][]int {
	parts := splitEnglishReading(reading)
	own := map[int][]int{}
	tail := map[int][]int{}
	lastPart := -1
	for i, piece := range pieces {
		tokens := make([]int, 0)
		for token := timing.PieceTokens[i][0]; token < timing.PieceTokens[i][1]; token++ {
			tokens = append(tokens, token)
		}
		if piece.Part >= 0 {
			lastPart = piece.Part
			own[piece.Part] = append(own[piece.Part], tokens...)
		} else if lastPart >= 0 {
			tail[lastPart] = append(tail[lastPart], tokens...)
		}
	}
	if lastPart >= 0 && len(timing.Durations) > 0 {
		tail[lastPart] = append(tail[lastPart], len(timing.Durations)-1)
		if last := len(parts) - 1; last > lastPart && parts[last] == "SP" {
			own[last] = append(own[last], tail[lastPart]...)
			tail[lastPart] = nil
		}
	}
	result := make([][]int, len(morae))
	pause := 0
	var pauseParts []int
	for index, part := range parts {
		if part == "SP" {
			pauseParts = append(pauseParts, index)
		}
	}
	for start := 0; start < len(morae); {
		mora := morae[start]
		if mora.Pause {
			if pause < len(pauseParts) {
				part := pauseParts[pause]
				result[start] = append(append([]int(nil), own[part]...), tail[part]...)
			}
			pause++
			start++
			continue
		}
		end := start
		for end+1 < len(morae) && !morae[end+1].Pause && morae[end+1].WordIndex == mora.WordIndex {
			end++
		}
		tokens := own[mora.WordIndex]
		if assigned := englishWordAssignment(pieces, timing, tokens, morae[start:end+1]); assigned != nil {
			for i, m := range assigned {
				result[start+i] = m
			}
			result[end] = append(result[end], tail[mora.WordIndex]...)
			start = end + 1
			continue
		}
		weights := make([]int, end-start+1)
		total := 0
		for i := range weights {
			weights[i] = max(1, len(morae[start+i].Phones))
			total += weights[i]
		}
		cumulative, at := 0, 0
		for i := range weights {
			cumulative += weights[i]
			next := int(math.Round(float64(len(tokens)) * float64(cumulative) / float64(total)))
			result[start+i] = append([]int(nil), tokens[at:next]...)
			at = next
		}
		result[end] = append(result[end], tail[mora.WordIndex]...)
		start = end + 1
	}
	return result
}

func splitEnglishReading(reading string) []string {
	var parts []string
	for _, part := range strings.Split(reading, "|") {
		if fields := strings.Fields(part); len(fields) > 0 {
			parts = append(parts, strings.Join(fields, " "))
		}
	}
	return parts
}

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
func alignEnglishWord(symbols []string, phones []linguistics.Phone) []int {
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
func englishWordAssignment(pieces []Piece, timing Timing, tokens []int, morae []linguistics.Mora) [][]int {
	var phones []linguistics.Phone
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
func tokenSymbol(timing Timing, pieces []Piece, token int) string {
	for i, span := range timing.PieceTokens {
		if token >= span[0] && token < span[1] {
			if runes := []rune(pieces[i].Symbols); token-span[0] < len(runes) {
				return string(runes[token-span[0]])
			}
		}
	}
	return ""
}
