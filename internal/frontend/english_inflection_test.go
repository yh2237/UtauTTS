package frontend

import (
	"strings"
	"testing"
)

func TestEnglishInflectionAllomorphs(t *testing.T) {
	for _, tc := range []struct{ word, want string }{
		{"cat's", "K AE1 T S"},
		{"dog's", "D AO1 G Z"},
		{"bus's", "B AH1 S IH0 Z"},
		{"cats'", "K AE1 T S"},
		{"walked", "W AO1 K T"},
		{"played", "P L EY1 D"},
		{"wanted", ""}, // "want" and the name "wante" are ambiguous.
		{"lifted", "L IH1 F T IH0 D"},
		{"running", "R AH1 N IH0 NG"},
		{"making", ""}, // Both "mak" and "make" occur in CMUdict.
		{"tried", "T R AY1 D"},
		{"babies", "B EY1 B IY0 Z"},
	} {
		t.Run(tc.word, func(t *testing.T) {
			got, err := englishInflectedPronunciation(tc.word)
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestEnglishInflectionFallsBackOnlyForDictionaryBackedStems(t *testing.T) {
	for _, word := range []string{"qzxqzx's", "qzxqzxing", "hello", "ss"} {
		if got, err := englishInflectedPronunciation(word); err != nil || got != "" {
			t.Fatalf("%s: got %q, %v", word, got, err)
		}
	}
	// A possessive of a technical word should retain its dictionary stress.
	word := "internationalization's"
	if direct, err := lookupEnglishDictionary(word); err != nil || direct != "" {
		t.Fatalf("test requires a missing inflected entry: %q, %v", direct, err)
	}
	base, err := lookupEnglishDictionary("internationalization")
	if err != nil || !strings.Contains(base, "1") {
		t.Fatalf("missing stressed stem: %q, %v", base, err)
	}
	got, err := englishWordPronunciation(word)
	if err != nil || got != base+" Z" {
		t.Fatalf("got %q, %v; stem %q", got, err, base)
	}
	for _, word := range []string{"does", "said", "read", "making"} {
		want, _ := lookupEnglishDictionary(word)
		got, err := englishWordPronunciation(word)
		if err != nil || got != want {
			t.Fatalf("exact dictionary entry changed for %s: %q, %v", word, got, err)
		}
	}
}
