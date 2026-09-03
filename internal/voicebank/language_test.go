package voicebank

import (
	"testing"

	"utautts/internal/oto"
)

func TestSuggestedLanguage(t *testing.T) {
	tests := []struct {
		name, language, phonemizer string
		aliases                    []string
	}{
		{"delta", "en", "en-delta", []string{"- h@", "h{", "@ l"}},
		{"vccv", "en", "en-vccv", []string{"-h@", "-b&"}},
		{"arpasing", "en", "en-arpasing", []string{"- hh", "hh ah"}},
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

func TestSuggestedLanguageUsesOpenUtauPhonemizer(t *testing.T) {
	bank := &Bank{DefaultPhonemizer: "OpenUtau.Plugin.Builtin.ChineseCVVCPhonemizer"}
	language, phonemizer := bank.SuggestedLanguage()
	if language != "zh" || phonemizer != "zh-cvvc" {
		t.Fatalf("got %s/%s", language, phonemizer)
	}
}
