package en

import "github.com/yh2237/UtauTTS/internal/g2p"

// lookupEnglishDictionaryは、CMUdict→LibriSpeechの順で英語の読みを引く。
func lookupEnglishDictionary(word string) (string, error) {
	entry, err := g2p.LookupEnglish(word)
	return entry.Phones, err
}
