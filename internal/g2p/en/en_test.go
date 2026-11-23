package en

import (
	"strings"
	"testing"

	"github.com/yh2237/UtauTTS/internal/g2p"
	"github.com/yh2237/UtauTTS/internal/linguistics"
)

func TestLookupEnglishSources(t *testing.T) {
	tests := map[string]struct {
		phones string
		source g2p.Source
	}{
		"google":    {"G UW1 G AH0 L", g2p.SourceCMU},
		"untended":  {"AH0 N T EH1 N D IH0 D", g2p.SourceLibriSpeech},
		"reckoners": {"R EH1 K AH0 N ER0 Z", g2p.SourceLibriSpeech},
		"faynie's":  {"F EY1 N IY0 Z", g2p.SourceLibriSpeech},
		"thakane":   {"TH AA1 K EY1 N", g2p.SourceLibriSpeech},
	}
	for word, want := range tests {
		entry, err := LookupEnglish(word)
		if err != nil {
			t.Fatalf("%s: %v", word, err)
		}
		if entry.Phones != want.phones || entry.Source != want.source {
			t.Errorf("%s = %q (%s), want %q (%s)", word, entry.Phones, entry.Source, want.phones, want.source)
		}
	}
}

func TestEnglishNumberWordsAndNormalize(t *testing.T) {
	tests := map[string]string{
		"21":      "twenty one",
		"2024":    "two thousand twenty four",
		"3.50":    "three point five zero",
		"007":     "zero zero seven",
		"1000000": "one million",
	}
	for input, want := range tests {
		if got := EnglishNumber(input); got != want {
			t.Errorf("EnglishNumber(%q) = %q, want %q", input, got, want)
		}
	}
	for input, want := range map[string]string{"3:15": "three fifteen", "3:05": "three oh five", "120": "one hundred twenty", "0.25": "zero point two five", "007": "zero zero seven"} {
		got := strings.Join(strings.Fields(Normalize(input)), " ")
		if got != want {
			t.Errorf("%q: %q want %q", input, got, want)
		}
	}
	got, err := Parse(linguistics.PhonemizerEnglish, "Meet me at 3:15.", "", nil)
	if err != nil || !strings.Contains(got.Reading, "TH R IY1") {
		t.Fatalf("number lost: %q %v", got.Reading, err)
	}
}

func TestEnglishReadUsesUnambiguousVerbContext(t *testing.T) {
	for _, tc := range []struct{ text, reading string }{
		{"Please read this text aloud.", "R IY1 D"},
		{"Are you ready to read the next chapter?", "R IY1 D"},
		{"Read this book.", "R IY1 D"},
		{"I will read this book.", "R IY1 D"},
		{"I did read this book.", "R IY1 D"},
		{"I have read this book.", "R EH1 D"},
		{"I read it yesterday.", "R EH1 D"},
	} {
		got, err := Parse(linguistics.PhonemizerEnglishDelta, tc.text, "", nil)
		if err != nil || !strings.Contains(got.Reading, tc.reading) {
			t.Fatalf("%q = %q, %v", tc.text, got.Reading, err)
		}
	}
	got, err := Parse(linguistics.PhonemizerEnglishVCCV, "Please read this.", "", map[string]string{"read": "R EH1 D"})
	if err != nil || !strings.Contains(got.Reading, "R EH1 D") {
		t.Fatalf("dictionary override lost: %q, %v", got.Reading, err)
	}
	got, err = Parse(linguistics.PhonemizerEnglishDelta, "Please read this.", "R EH1 D", nil)
	if err != nil || got.Reading != "R EH1 D" {
		t.Fatalf("explicit reading lost: %q, %v", got.Reading, err)
	}
}

func TestEnglishOOVAcrossPhonemizersAndDictionaryOverride(t *testing.T) {
	for _, phonemizer := range []string{
		linguistics.PhonemizerEnglish,
		linguistics.PhonemizerEnglishDelta,
		linguistics.PhonemizerEnglishVCCV,
		linguistics.PhonemizerEnglishCV,
	} {
		got, err := Parse(phonemizer, "phlame", "", nil)
		if err != nil || got.Reading != "F L EY M" || len(got.Morae) == 0 {
			t.Fatalf("%s: %q %v", phonemizer, got.Reading, err)
		}
		for _, unit := range got.Morae {
			if unit.StressKnown {
				t.Fatalf("unknown word gained stress: %+v", unit)
			}
		}
		override, err := Parse(phonemizer, "phlame", "", map[string]string{"phlame": "F L AE1 M"})
		if err != nil || override.Reading != "F L AE1 M" {
			t.Fatalf("%s override: %q %v", phonemizer, override.Reading, err)
		}
		explicit, err := Parse(phonemizer, "phlame", "F L IY1 M", nil)
		if err != nil || explicit.Reading != "F L IY1 M" {
			t.Fatalf("%s explicit reading: %q %v", phonemizer, explicit.Reading, err)
		}
	}
}

func TestDictionaryPreservesStressBeyondMiniDictionary(t *testing.T) {
	for _, word := range []string{"hello", "international", "pronunciation", "beautiful"} {
		got, err := Parse(linguistics.PhonemizerEnglish, word, "", nil)
		if err != nil || !strings.Contains(got.Reading, "1") {
			t.Fatalf("%s: %q %v", word, got.Reading, err)
		}
	}
	got, err := Parse(linguistics.PhonemizerEnglish, "hello", "", map[string]string{"hello": "HH EH1 L OW0"})
	if err != nil || got.Reading != "HH EH1 L OW0" {
		t.Fatalf("user dictionary: %q %v", got.Reading, err)
	}
}

func TestEnglishPhoneCodasAndTiming(t *testing.T) {
	for _, phonemizer := range []string{linguistics.PhonemizerEnglish, linguistics.PhonemizerEnglishCV} {
		got, err := Parse(phonemizer, "", "T EH1 S T | R IY1 D | SP", nil)
		if err != nil {
			t.Fatal(err)
		}
		for i, role := range []string{"onset", "nucleus", "coda", "coda", "onset", "nucleus", "coda"} {
			if got.Morae[i].Phones[0].Role != role {
				t.Fatalf("%s phone %d: got %s, want %s", phonemizer, i, got.Morae[i].Phones[0].Role, role)
			}
		}
	}
	got, err := Parse(linguistics.PhonemizerEnglish, "", "S T AE1", nil)
	if err != nil || got.Morae[0].DurationScale <= got.Morae[1].DurationScale {
		t.Fatalf("%+v %v", got.Morae, err)
	}
	if _, err := Parse(linguistics.PhonemizerEnglish, "", "K INVALID AE1", nil); err == nil {
		t.Fatal("invalid phone accepted")
	}
	if _, err := Parse(linguistics.PhonemizerEnglishCV, "", "K INVALID AE1", nil); err == nil {
		t.Fatal("invalid phone accepted for cv")
	}
}

func TestEnglishLexicalStressAndSyllables(t *testing.T) {
	for _, phone := range []struct {
		vowel, symbol string
		stress        int
		stressKnown   bool
	}{
		{"AH0", "ah", 0, true}, {"AH1", "ah", 1, true}, {"AH2", "ah", 2, true},
		{"AH", "ah", 0, false}, {"AX", "ax", 0, false},
	} {
		got, err := Parse(linguistics.PhonemizerEnglishDelta, "", "HH "+phone.vowel+" N | SP", nil)
		if err != nil {
			t.Fatal(err)
		}
		if got.Reading != "HH "+phone.vowel+" N | SP" || len(got.Morae) != 2 || !got.Morae[1].Pause {
			t.Fatalf("%s: reading=%q morae=%+v", phone.vowel, got.Reading, got.Morae)
		}
		m := got.Morae[0]
		if m.Phones[1].Symbol != phone.symbol || m.Stress != phone.stress || m.StressKnown != phone.stressKnown {
			t.Fatalf("lexical stress/phone metadata changed: %s %+v", phone.vowel, m)
		}
	}
}
