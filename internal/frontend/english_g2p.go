package frontend

import "github.com/yh2237/UtauTTS/internal/g2p/en"

// englishWordPronunciationInContextは、前後の語で同綴異音語を選ぶ。
func englishWordPronunciationInContext(word, previous, next string) (string, error) {
	reading, err := en.ResolveInContext(word, previous, next)
	return reading.Phones, err
}
