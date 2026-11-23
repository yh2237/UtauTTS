package tts

import (
	"testing"

	"github.com/yh2237/UtauTTS/internal/linguistics"
)

func TestJapanesePhoneTimelineHasDefaultWeights(t *testing.T) {
	morae := []linguistics.Mora{{Text: "か", Consonant: "k", Vowel: "a"}}
	japaneseSpeechPhones(morae)
	weights := languagePhoneWeights(linguistics.LanguageJapanese, morae)
	if len(morae[0].Phones) != 2 || len(weights[0]) != 2 {
		t.Fatalf("phones=%v weights=%v", morae[0].Phones, weights)
	}
}

func TestJapaneseLanguagePhoneTimingIsGated(t *testing.T) {
	if shouldUseLanguagePhoneTiming(linguistics.LanguageJapanese, false) || !shouldUseLanguagePhoneTiming(linguistics.LanguageJapanese, true) {
		t.Fatal("Japanese phone timing must be limited to single CV banks")
	}
}
