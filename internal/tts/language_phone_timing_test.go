package tts

import (
	"testing"

	"utautts/internal/frontend"
)

func TestEnglishPhoneTimingPreservesFinalStop(t *testing.T) {
	mora := frontend.Mora{Language: frontend.LanguageEnglish, Stress: 1, StressKnown: true, Phones: []frontend.Phone{
		{Symbol: "k", Role: "onset"}, {Symbol: "ah", Role: "nucleus"}, {Symbol: "p", Role: "coda"},
	}}
	weights := languagePhoneWeights(frontend.LanguageEnglish, []frontend.Mora{mora})[0]
	if len(weights) != 3 || weights[2] <= frontend.PhoneWeight("p", "coda") {
		t.Fatalf("English phone weights = %v", weights)
	}
	spans := phoneSpansFromWeights(weights, 180)
	if spans[2] < 35 {
		t.Fatalf("final p duration = %.1f ms, want at least 35 ms", spans[2])
	}
}

func TestLanguagePredictionsControlEnergy(t *testing.T) {
	english := englishPredictions([]frontend.Mora{
		{Vowel: "ah", Stress: 1, StressKnown: true},
		{Vowel: "ax", Stress: 0, StressKnown: true},
	})
	if english[0].EnergyFactor <= english[1].EnergyFactor {
		t.Fatalf("English energy = %+v", english)
	}
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
	tests := []struct {
		name         string
		speechTiming bool
		targetPrior  bool
		singleCV     bool
		want         bool
	}{
		{name: "ordinary continuous speech", want: false},
		{name: "explicit speech timing", speechTiming: true, want: true},
		{name: "target prior", targetPrior: true, want: true},
		{name: "single cv bank", singleCV: true, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldUseLanguagePhoneTiming(frontend.LanguageJapanese, test.speechTiming, test.targetPrior, test.singleCV); got != test.want {
				t.Fatalf("use phone timing = %v, want %v", got, test.want)
			}
		})
	}
	if !shouldUseLanguagePhoneTiming(frontend.LanguageEnglish, false, false, false) {
		t.Fatal("English phone timing was disabled")
	}
}
