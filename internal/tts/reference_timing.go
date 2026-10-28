//go:build !js

package tts

import (
	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/refspeech"
)

// simpleMoraTokensは、断片とモーラが1対1で並ぶ言語（中国語など）で、
// Kokoroの入力記号をモーラへ割り当てる。先頭の境界記号はどのモーラにも入れず、
// 末尾の境界記号は最後のモーラに入れる。
func simpleMoraTokens(pieces []refspeech.Piece, timing refspeech.Timing, morae []frontend.Mora) [][]int {
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
func partMoraTokens(pieces []refspeech.Piece, timing refspeech.Timing, morae []frontend.Mora) [][]int {
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

// referenceMoraTokensは、言語に合った割り当て方で、Kokoroの入力記号をモーラへ割り当てる。
func referenceMoraTokens(language string, pieces []refspeech.Piece, timing refspeech.Timing, reading string, morae []frontend.Mora) [][]int {
	return referenceProfileFor(language).moraTokens(pieces, timing, reading, morae)
}
