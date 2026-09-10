package frontend

import (
	"strings"
	"testing"
)

func TestEnglishSpellingFallback(t *testing.T) {
	for _, tc := range []struct{ word, want string }{
		{"phlame", "F L EY M"},
		{"quindle", "K W IH N D AH L"},
		{"thwip", "TH W IH P"},
		{"smeesh", "S M IY SH"},
		{"chazzing", "CH AE Z IH NG"},
		{"florp", "F L AO R P"},
		{"blim", "B L IH M"},
		{"knorp", "N AO R P"},
		{"wrife", "R AY F"},
		{"snigh", "S N AY"},
		{"voction", "V AA K SH AH N"},
	} {
		t.Run(tc.word, func(t *testing.T) {
			got, err := englishRulePronunciation(tc.word)
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
			if strings.ContainsAny(got, "012") {
				t.Fatalf("spelling fallback invented lexical stress: %q", got)
			}
		})
	}
}

func TestEnglishSpellingRejectsUnsupportedInput(t *testing.T) {
	for _, word := range []string{"", "'", "123", "hello!", "二", "a b", "naïve"} {
		if _, err := englishRulePronunciation(word); err == nil {
			t.Errorf("accepted %q", word)
		}
	}
}

func TestEnglishPrefixPreservesStemPronunciation(t *testing.T) {
	for _, tc := range []struct{ word, stem, prefix string }{
		{"microblogging", "blogging", "M AY2 K R OW0"},
		{"unmuting", "muting", "AH0 N"},
		{"preloading", "loading", "P R IY0"},
		{"cybersecurity", "security", "S AY2 B ER0"},
	} {
		if direct, err := lookupEnglishDictionary(tc.word); err != nil || direct != "" {
			t.Fatalf("test requires an OOV word %q: %q %v", tc.word, direct, err)
		}
		stem, err := lookupEnglishDictionary(tc.stem)
		if err != nil || !strings.Contains(stem, "1") {
			t.Fatalf("missing stressed stem: %q %v", stem, err)
		}
		got, err := englishWordPronunciation(tc.word)
		if err != nil || got != tc.prefix+" "+stem {
			t.Errorf("%s: %q %v", tc.word, got, err)
		}
	}
	if got, err := englishPrefixedPronunciation("microqzxqzx"); err != nil || got != "" {
		t.Fatalf("unknown stem accepted: %q %v", got, err)
	}
}

func TestEnglishOOVAcrossPhonemizersAndDictionaryOverride(t *testing.T) {
	for _, parse := range []func(string, string, map[string]string) (string, []Mora, error){ParseEnglishARPAsing, ParseEnglishDelta, ParseEnglishVCCV} {
		reading, units, err := parse("phlame", "", nil)
		if err != nil || reading != "F L EY M" || len(units) == 0 {
			t.Fatalf("%q %v", reading, err)
		}
		for _, unit := range units {
			if unit.StressKnown {
				t.Fatalf("unknown word gained stress: %+v", unit)
			}
		}
		reading, _, err = parse("phlame", "", map[string]string{"phlame": "F L AE1 M"})
		if err != nil || reading != "F L AE1 M" {
			t.Fatalf("override: %q %v", reading, err)
		}
		reading, _, err = parse("phlame", "F L IY1 M", nil)
		if err != nil || reading != "F L IY1 M" {
			t.Fatalf("explicit reading: %q %v", reading, err)
		}
	}
}
