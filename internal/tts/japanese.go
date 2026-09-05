package tts

import (
	"strings"
	"unicode"
)

// 疑問上昇は各文ではなく、最後の句だけに適用する。
func finalPhraseIsQuestion(text string) bool {
	trimmed := strings.TrimRightFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune("」』）)]\"'。.!！", r)
	})
	return strings.HasSuffix(trimmed, "?") || strings.HasSuffix(trimmed, "？")
}
