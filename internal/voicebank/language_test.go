package voicebank

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yh2237/UtauTTS/internal/oto"
)

func TestSuggestedLanguage(t *testing.T) {
	t.Run("inventory aliases", testSuggestedLanguage)
	t.Run("OpenUtau phonemizer", testSuggestedLanguageUsesOpenUtauPhonemizer)
	t.Run("OpenUtau phonemizer files", testSuggestedLanguageUsesPhonemizerFiles)
	t.Run("pitch affixes", testSuggestedLanguageIgnoresPitchAffixes)
}

func testSuggestedLanguageIgnoresPitchAffixes(t *testing.T) {
	aliases := []string{"- hVD#4", "- h@D#4", "V lD#4", "@ lD#4", "h{D#4", "- h{D#4", "- niD#4"}
	subbank := &Bank{Entries: map[string][]oto.Entry{}, Subbanks: []Subbank{{Suffix: "D#4"}}}
	prefixMap := &Bank{Entries: map[string][]oto.Entry{}, PrefixMap: map[string]Affix{"D#4": {Suffix: "D#4"}}}
	for name, bank := range map[string]*Bank{"subbank": subbank, "prefix.map": prefixMap} {
		for _, alias := range aliases {
			bank.Entries[alias] = []oto.Entry{{Alias: alias}}
		}
		if language, phonemizer := bank.SuggestedLanguage(); language != "en" || phonemizer != "en-delta" {
			t.Fatalf("%s: got %s/%s", name, language, phonemizer)
		}
	}
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
		{"adachi rei english c+v", "en", "en-cv", []string{"aa", "ah", "ao", "hh", "jh", "ng", "k", "uw-"}},
		{"chinese", "zh", "zh-cvvc", []string{"- ni", "hao"}},
		{"japanese", "ja", "ja-kana", []string{"- あ", "あ"}},
		{"japanese romaji", "ja", "ja-kana", []string{"a", "ka", "- ka", "a ka", "n", "nga"}},
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
	xsampa := &Bank{DefaultPhonemizer: "OpenUtau.Plugin.Builtin.EnXSampaPhonemizer"}
	language, phonemizer = xsampa.SuggestedLanguage()
	if language != "en" || phonemizer != "en-delta" {
		t.Fatalf("x-sampa got %s/%s", language, phonemizer)
	}
}

func testSuggestedLanguageUsesPhonemizerFiles(t *testing.T) {
	for file, want := range map[string]string{"envccv.yaml": "en-vccv", "en-xsampa.yaml": "en-delta", "en-cPv.yaml": "en-cv"} {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, file), []byte("symbols: []\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		bank := &Bank{Root: root, Entries: map[string][]oto.Entry{"a": {{Alias: "a"}}}}
		if language, phonemizer := bank.SuggestedLanguage(); language != "en" || phonemizer != want {
			t.Fatalf("%s: got %s/%s, want en/%s", file, language, phonemizer, want)
		}
	}
}

func TestSuggestedLanguageIgnoresInlineToneSuffix(t *testing.T) {
	bank := &Bank{Entries: map[string][]oto.Entry{}}
	for _, alias := range []string{"- hhB3", "hh ahB3", "ah -B3", "- aaB3", "aaB3", "- aaD4", "hh ahD4"} {
		bank.Entries[alias] = []oto.Entry{{Alias: alias}}
	}
	if language, phonemizer := bank.SuggestedLanguage(); language != "en" || phonemizer != "en-arpasing" {
		t.Fatalf("got %s/%s", language, phonemizer)
	}
	if base, suffix, ok := splitToneSuffix("- hhB3"); !ok || base != "- hh" || suffix != "B3" {
		t.Fatalf("splitToneSuffix = %q %q %v", base, suffix, ok)
	}
	if _, _, ok := splitToneSuffix("- hh"); ok {
		t.Fatal("a plain alias has a tone suffix")
	}
}
