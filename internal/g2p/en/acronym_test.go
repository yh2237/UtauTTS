package en

import "testing"

func TestEnglishAcronymsSpellLetters(t *testing.T) {
	tests := map[string]string{
		"AI":    "EY1 AY2",
		"USB":   "Y UW1 EH2 S B IY2",
		"NASA":  "N AE1 S AH0",
		"HELLO": "HH AH0 L OW1",
	}
	for word, want := range tests {
		got, err := englishWordPronunciation(word)
		if err != nil {
			t.Fatalf("%s: %v", word, err)
		}
		if got != want {
			t.Errorf("%s = %q, want %q", word, got, want)
		}
	}
}
