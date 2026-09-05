package tts

import (
	"strings"
	"unicode"
)

// The contour's question flag applies to its final phrase, not every sentence.
func finalPhraseIsQuestion(text string) bool {
	trimmed := strings.TrimRightFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune("」』）)]\"'。.!！", r)
	})
	return strings.HasSuffix(trimmed, "?") || strings.HasSuffix(trimmed, "？")
}
