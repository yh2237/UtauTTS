package frontend

import "github.com/yh2237/UtauTTS/internal/g2p"

// spanishNumberWordsは数字の並びをスペイン語の語にする。実体はinternal/g2pにある。
func spanishNumberWords(token string) string {
	return g2p.SpanishNumberWords(token)
}
