package frontend

import "github.com/yh2237/UtauTTS/internal/g2p/en"

// englishWordPronunciationは、英語の1語を優先順位の高い候補から読む。
func englishWordPronunciation(word string) (string, error) {
	reading, err := en.Resolve(word)
	return reading.Phones, err
}

// englishWordPronunciationInContextは、前後の語で同綴異音語を選ぶ。
func englishWordPronunciationInContext(word, previous, next string) (string, error) {
	reading, err := en.ResolveInContext(word, previous, next)
	return reading.Phones, err
}
