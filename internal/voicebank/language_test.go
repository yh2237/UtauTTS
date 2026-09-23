package voicebank

import (
	"testing"

	"utautts/internal/oto"
)

func TestSuggestedLanguage(t *testing.T) {
	t.Run("inventory aliases", testSuggestedLanguage)
	t.Run("OpenUtau phonemizer", testSuggestedLanguageUsesOpenUtauPhonemizer)
}

func testSuggestedLanguage(t *testing.T) {
	tests := []struct {
		name, language, phonemizer string
		aliases                    []string
	}{
		{"delta", "en", "en-delta", []string{"- h@", "h{", "@ l"}},
		{"teto delta", "en", "en-delta", []string{"- hV", "- h@", "V l", "@ l", "h{", "- h{"}},
		{"nui neo delta", "en", "en-delta", []string{"- h@", "@ l", "h{", "- h{"}},
		{"vccv", "en", "en-vccv", []string{"-h@", "-b&"}},
		{"nui vccv", "en", "en-vccv", []string{"-h@", "-hA", "-b&"}},
		{"arpasing", "en", "en-arpasing", []string{"- hh", "hh ah"}},
		{"cv", "en", "en-cv", []string{"- aa", "aa", "aa -"}},
		{"veriacveng cv", "en", "en-cv", []string{"- aa", "- ah", "- ao", "aa -", "ah -", "ao -"}},
		{"chinese", "zh", "zh-cvvc", []string{"- ni", "hao"}},
		{"japanese", "ja", "ja-kana", []string{"- あ", "あ"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bank := &Bank{Entries: map[string][]oto.Entry{}}
			for _, alias := range test.aliases {
				bank.Entries[alias] = []oto.Entry{{Alias: alias}}
			}
			language, phonemizer := bank.SuggestedLanguage()
			if language != test.language || phonemizer != test.phonemizer {
				t.Fatalf("got %s/%s", language, phonemizer)
			}
		})
	}
}

func testSuggestedLanguageUsesOpenUtauPhonemizer(t *testing.T) {
	bank := &Bank{DefaultPhonemizer: "OpenUtau.Plugin.Builtin.ChineseCVVCPhonemizer"}
	language, phonemizer := bank.SuggestedLanguage()
	if language != "zh" || phonemizer != "zh-cvvc" {
		t.Fatalf("got %s/%s", language, phonemizer)
	}
	cpv := &Bank{DefaultPhonemizer: "OpenUtau.Plugin.Builtin.EnglishCpVPhonemizerBeta"}
	language, phonemizer = cpv.SuggestedLanguage()
	if language != "en" || phonemizer != "en-cv" {
		t.Fatalf("got %s/%s", language, phonemizer)
	}
}
