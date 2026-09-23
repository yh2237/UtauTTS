package tts

import (
	"os"
	"path/filepath"
	"testing"

	"utautts/internal/frontend"
	"utautts/internal/oto"
	"utautts/internal/voicebank"
)

func bankWithAliases(aliases ...string) *voicebank.Bank {
	bank := &voicebank.Bank{Entries: map[string][]oto.Entry{}}
	for _, alias := range aliases {
		bank.Entries[alias] = []oto.Entry{{Alias: alias}}
	}
	return bank
}

// phonemizer未指定のとき、音源のalias在庫から推定したphonemizerを使う。
func TestResolvePronunciationUsesVoicebankSuggestedPhonemizer(t *testing.T) {
	tests := []struct {
		name           string
		aliases        []string
		wantPhonemizer string
	}{
		{"delta", []string{"- hV", "- h@", "V l", "@ l", "h{", "- h{"}, frontend.PhonemizerEnglishDelta},
		{"vccv", []string{"-h@", "-hA", "-b&"}, frontend.PhonemizerEnglishVCCV},
		{"cv", []string{"- aa", "- ah", "- ao", "aa -", "ah -", "ao -"}, frontend.PhonemizerEnglishCV},
		{"arpasing", []string{"- hh", "hh ah", "ah l"}, frontend.PhonemizerEnglish},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := Config{
				Text: "Hello", Language: "en", Reading: "HH AH0 L OW1",
				Voicebank: bankWithAliases(test.aliases...),
			}
			language, phonemizer, _, _, err := resolvePronunciation(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if language != "en" || phonemizer != test.wantPhonemizer {
				t.Fatalf("got %s/%s, want en/%s", language, phonemizer, test.wantPhonemizer)
			}
		})
	}
}

// 明示phonemizerは音源推定より優先する。
func TestResolvePronunciationPrefersExplicitPhonemizer(t *testing.T) {
	cfg := Config{
		Text: "Hello", Language: "en", Reading: "HH AH0 L OW1",
		Phonemizer: frontend.PhonemizerEnglishVCCV,
		Voicebank:  bankWithAliases("- hV", "- h@", "V l", "@ l", "h{", "- h{"),
	}
	language, phonemizer, _, _, err := resolvePronunciation(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if language != "en" || phonemizer != frontend.PhonemizerEnglishVCCV {
		t.Fatalf("got %s/%s, want en/%s", language, phonemizer, frontend.PhonemizerEnglishVCCV)
	}
}

// 明示言語が推定言語と食い違うときは明示言語の既定へ戻す。
func TestResolvePronunciationFallsBackWhenLanguageDiffers(t *testing.T) {
	cfg := Config{
		Reading:   "アカ",
		Language:  "ja",
		Voicebank: bankWithAliases("- hV", "- h@", "V l", "@ l", "h{", "- h{"),
	}
	language, phonemizer, _, _, err := resolvePronunciation(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if language != "ja" || phonemizer != frontend.PhonemizerJapanese {
		t.Fatalf("got %s/%s, want ja/%s", language, phonemizer, frontend.PhonemizerJapanese)
	}
}

// VoicebankPathしか無い経路でも音源を読み込んで推定する。
func TestResolvePronunciationUsesVoicebankPath(t *testing.T) {
	root := t.TempDir()
	otoData := "a.wav=- hV,0,0,0,0,0\n" +
		"b.wav=- h@,0,0,0,0,0\n" +
		"c.wav=V l,0,0,0,0,0\n" +
		"d.wav=@ l,0,0,0,0,0\n" +
		"e.wav=h{,0,0,0,0,0\n" +
		"f.wav=- h{,0,0,0,0,0\n"
	if err := os.WriteFile(filepath.Join(root, "oto.ini"), []byte(otoData), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		Text: "Hello", Language: "en", Reading: "HH AH0 L OW1", VoicebankPath: root,
	}
	language, phonemizer, _, _, err := resolvePronunciation(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if language != "en" || phonemizer != frontend.PhonemizerEnglishDelta {
		t.Fatalf("got %s/%s, want en/%s", language, phonemizer, frontend.PhonemizerEnglishDelta)
	}
}
