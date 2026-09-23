package tts

import (
	"testing"

	"utautts/internal/frontend"
	"utautts/internal/prosody"
	"utautts/internal/render"
)

// 言語ごとに対応するprofileが選ばれる。
func TestLanguageProfileForSelectsByLanguage(t *testing.T) {
	tests := []struct {
		language string
		want     string
	}{
		{frontend.LanguageJapanese, frontend.LanguageJapanese},
		{frontend.LanguageEnglish, frontend.LanguageEnglish},
		{frontend.LanguageChinese, frontend.LanguageChinese},
		{"", frontend.LanguageJapanese},
	}
	for _, test := range tests {
		if got := languageProfileFor(test.language).Language(); got != test.want {
			t.Fatalf("languageProfileFor(%q).Language() = %q, want %q", test.language, got, test.want)
		}
	}
}

// profileごとに基本予測の有無が切り替わる。
func TestLanguageProfilePredictSelection(t *testing.T) {
	morae := []frontend.Mora{{Vowel: "a", Stress: 1, StressKnown: true, Tone: 4}}
	if predictions := languageProfileFor(frontend.LanguageEnglish).Predict(morae); len(predictions) != len(morae) {
		t.Fatalf("English predictions = %#v", predictions)
	}
	if predictions := languageProfileFor(frontend.LanguageChinese).Predict(morae); len(predictions) != len(morae) {
		t.Fatalf("Chinese predictions = %#v", predictions)
	}
	if predictions := languageProfileFor(frontend.LanguageJapanese).Predict(morae); predictions != nil {
		t.Fatalf("Japanese predictions = %#v, want nil", predictions)
	}
}

// 言語ごとに規則ベースのF0曲線が選ばれ、日本語は自動曲線を持たない。
func TestLanguageProfilePitchCurveSelection(t *testing.T) {
	timings := []prosody.MoraTiming{{StartMS: 0, DurationMS: 120}, {StartMS: 120, DurationMS: 120}}

	english := []frontend.Mora{{Vowel: "ah", Stress: 1, StressKnown: true}, {Vowel: "ax", Stress: 0, StressKnown: true}}
	if curve, _ := languageProfileFor(frontend.LanguageEnglish).AutomaticPitchCurve(Config{ApplyPitch: true, IntonationStrength: 1}, nil, english, timings, 240); curve == nil {
		t.Fatal("English automatic pitch curve was not selected")
	}
	if curve, _ := languageProfileFor(frontend.LanguageEnglish).AutomaticPitchCurve(Config{}, nil, english, timings, 240); curve != nil {
		t.Fatal("English automatic pitch curve was selected while pitch was disabled")
	}

	mandarin := []frontend.Mora{{Tone: 3}, {Tone: 1}}
	curve, enable := languageProfileFor(frontend.LanguageChinese).AutomaticPitchCurve(Config{}, nil, mandarin, timings, 240)
	if curve == nil || !enable {
		t.Fatalf("Chinese tone curve = %#v enable=%v", curve, enable)
	}

	if curve, _ := languageProfileFor(frontend.LanguageJapanese).AutomaticPitchCurve(Config{ApplyPitch: true}, nil, english, timings, 240); curve != nil {
		t.Fatalf("Japanese automatic pitch curve = %#v, want nil", curve)
	}
}

// 日本語の境界音調はprofile経由で適用され、他言語は曲線を変えない。
func TestLanguageProfileBoundaryToneSelection(t *testing.T) {
	base := &render.PitchCurve{FrameMS: 10, Cents: []float64{0, 0, 0, 0, 0, 0}}
	japanese := languageProfileFor(frontend.LanguageJapanese).ApplyBoundaryTone(Config{}, base, 50, false)
	if japanese == base || japanese.Cents[len(japanese.Cents)-1] == 0 {
		t.Fatalf("Japanese boundary tone was not applied: %#v", japanese)
	}
	english := languageProfileFor(frontend.LanguageEnglish).ApplyBoundaryTone(Config{}, base, 50, false)
	if english != base {
		t.Fatal("English boundary tone changed the curve")
	}
}

// phone timingの有効条件が言語ごとに異なる。
func TestLanguageProfilePhoneTimingSelection(t *testing.T) {
	morae := []frontend.Mora{{Text: "か", Consonant: "k", Vowel: "a"}}
	japanese := languageProfileFor(frontend.LanguageJapanese)
	if weights, source := japanese.PhoneTiming(Config{}, morae, false); weights != nil || source != "" {
		t.Fatalf("Japanese phone timing = %#v/%q, want nil", weights, source)
	}
	if weights, source := japanese.PhoneTiming(Config{SpeechTiming: true}, morae, false); len(weights) != 1 || source != "language-phone-v1" {
		t.Fatalf("Japanese speech timing weights = %#v source=%q", weights, source)
	}
	single := []frontend.Mora{{Text: "か", Consonant: "k", Vowel: "a"}}
	if weights, _ := japanese.PhoneTiming(Config{}, single, true); len(weights) != 1 || len(single[0].Phones) != 2 {
		t.Fatalf("Japanese single-CV phone timing = %#v phones=%#v", weights, single[0].Phones)
	}
	englishMora := []frontend.Mora{{Phones: []frontend.Phone{{Symbol: "k", Role: "onset"}}}}
	if weights, source := languageProfileFor(frontend.LanguageEnglish).PhoneTiming(Config{}, englishMora, false); len(weights) != 1 || source != "language-phone-v1" {
		t.Fatalf("English phone timing = %#v/%q", weights, source)
	}
}
