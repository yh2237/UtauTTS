package en

import "testing"

func TestEnglishDictionaryFallsBackToLibriSpeech(t *testing.T) {
	tests := map[string]string{
		"untended":  "AH0 N T EH1 N D IH0 D",
		"reckoners": "R EH1 K AH0 N ER0 Z",
		"faynie's":  "F EY1 N IY0 Z",
		"thakane":   "TH AA1 K EY1 N",
		"google":    "G UW1 G AH0 L",
	}
	for word, want := range tests {
		got, err := lookupEnglishDictionary(word)
		if err != nil {
			t.Fatalf("%s: %v", word, err)
		}
		if got != want {
			t.Errorf("%s = %q, want %q", word, got, want)
		}
	}
}
