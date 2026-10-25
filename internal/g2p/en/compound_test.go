package en

import "testing"

func TestEnglishWordPronunciationUsesCompoundAndNeuralFallback(t *testing.T) {
	tests := map[string]string{
		"OpenAI":   "OW1 P AH0 N EY1 AY2",
		"Miku":     "M IY1 K UW1",
		"Zundamon": "Z AH0 N D AE1 M AH0 N",
		"Google":   "G UW1 G AH0 L",
		"ChatGPT":  "CH AE1 T JH IY1 P IY2 T IY2",
		"Nvidia":   "EH1 N V IH1 D IY0 AH0",
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
