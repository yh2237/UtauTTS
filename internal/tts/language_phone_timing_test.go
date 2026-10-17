package tts

import (
	"testing"

	"github.com/yh2237/UtauTTS/internal/frontend"
)

func TestLanguagePredictionsControlEnergy(t *testing.T) {
	mandarin := mandarinPredictions([]frontend.Mora{{Vowel: "a", Tone: 4}, {Vowel: "a", Tone: 5}})
	if mandarin[1].DurationFactor >= mandarin[0].DurationFactor || mandarin[1].EnergyFactor >= mandarin[0].EnergyFactor {
		t.Fatalf("Mandarin neutral tone = %+v", mandarin)
	}
}

func TestJapanesePhoneTimelineHasDefaultWeights(t *testing.T) {
	morae := []frontend.Mora{{Text: "か", Consonant: "k", Vowel: "a"}}
	japaneseSpeechPhones(morae)
	weights := languagePhoneWeights(frontend.LanguageJapanese, morae)
	if len(morae[0].Phones) != 2 || len(weights[0]) != 2 {
		t.Fatalf("phones=%v weights=%v", morae[0].Phones, weights)
	}
}

func TestJapaneseLanguagePhoneTimingIsGated(t *testing.T) {
	if shouldUseLanguagePhoneTiming(frontend.LanguageJapanese, false) || !shouldUseLanguagePhoneTiming(frontend.LanguageJapanese, true) {
		t.Fatal("Japanese phone timing must be limited to single CV banks")
	}
	if !shouldUseLanguagePhoneTiming(frontend.LanguageChinese, false) {
		t.Fatal("Chinese phone timing was disabled")
	}
}
