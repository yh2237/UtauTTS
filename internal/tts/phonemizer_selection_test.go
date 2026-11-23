package tts

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yh2237/UtauTTS/internal/linguistics"
	"github.com/yh2237/UtauTTS/internal/oto"
	"github.com/yh2237/UtauTTS/internal/voicebank"
)

func bankWithAliases(aliases ...string) *voicebank.Bank {
	bank := &voicebank.Bank{Entries: map[string][]oto.Entry{}}
	for _, alias := range aliases {
		bank.Entries[alias] = []oto.Entry{{Alias: alias}}
	}
	return bank
}

func TestResolvePronunciationUsesVoicebankSuggestedPhonemizer(t *testing.T) {
	tests := []struct {
		name           string
		aliases        []string
		wantPhonemizer string
	}{
		{"delta", []string{"- hV", "- h@", "V l", "@ l", "h{", "- h{"}, linguistics.PhonemizerEnglishDelta},
		{"vccv", []string{"-h@", "-hA", "-b&"}, linguistics.PhonemizerEnglishVCCV},
		{"cv", []string{"- aa", "- ah", "- ao", "aa -", "ah -", "ao -"}, linguistics.PhonemizerEnglishCV},
		{"arpasing", []string{"- hh", "hh ah", "ah l"}, linguistics.PhonemizerEnglish},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := Config{
				Text: "Hello", Language: "en", Reading: "HH AH0 L OW1",
				Voicebank: bankWithAliases(test.aliases...),
			}
			language, phonemizer, _, _, err := ResolvePronunciation(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if language != "en" || phonemizer != test.wantPhonemizer {
				t.Fatalf("got %s/%s, want en/%s", language, phonemizer, test.wantPhonemizer)
			}
		})
	}
}

func TestResolvePronunciationPrefersExplicitPhonemizer(t *testing.T) {
	cfg := Config{
		Text: "Hello", Language: "en", Reading: "HH AH0 L OW1",
		Phonemizer: linguistics.PhonemizerEnglishVCCV,
		Voicebank:  bankWithAliases("- hV", "- h@", "V l", "@ l", "h{", "- h{"),
	}
	language, phonemizer, _, _, err := ResolvePronunciation(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if language != "en" || phonemizer != linguistics.PhonemizerEnglishVCCV {
		t.Fatalf("got %s/%s, want en/%s", language, phonemizer, linguistics.PhonemizerEnglishVCCV)
	}
}

func TestResolvePronunciationFallsBackWhenLanguageDiffers(t *testing.T) {
	cfg := Config{
		Reading:   "アカ",
		Language:  "ja",
		Voicebank: bankWithAliases("- hV", "- h@", "V l", "@ l", "h{", "- h{"),
	}
	language, phonemizer, _, _, err := ResolvePronunciation(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if language != "ja" || phonemizer != linguistics.PhonemizerJapanese {
		t.Fatalf("got %s/%s, want ja/%s", language, phonemizer, linguistics.PhonemizerJapanese)
	}
}

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
	language, phonemizer, _, _, err := ResolvePronunciation(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if language != "en" || phonemizer != linguistics.PhonemizerEnglishDelta {
		t.Fatalf("got %s/%s, want en/%s", language, phonemizer, linguistics.PhonemizerEnglishDelta)
	}
}
