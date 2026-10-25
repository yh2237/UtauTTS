package en

import "testing"

func TestEnglishNeuralPronunciationMatchesReferenceModel(t *testing.T) {
	tests := map[string]string{
		"openai":        "OW0 P EY0 N AA1",
		"miku":          "M IH1 K UW0",
		"nvidia":        "V AH0 N D IY1 AH0",
		"google":        "G UW1 G AH0 L",
		"chatgpt":       "CH AE1 T P T",
		"activationist": "AE2 K T IH0 V EY1 SH AH0 N IH0 S T",
		"zundamon":      "Z AH0 N D AE1 M AH0 N",
		"sugoooii":      "S UW2 JH OW0 W EY1",
		"python":        "P AY1 TH AH0 N",
		"github":        "G IH1 TH AH0 B",
	}
	for word, want := range tests {
		got, ok := englishNeuralPronunciation(word)
		if !ok {
			t.Fatalf("%s: no pronunciation", word)
		}
		if got != want {
			t.Errorf("%s = %q, want %q", word, got, want)
		}
	}
}

func TestEnglishNeuralPronunciationRejectsNonLetters(t *testing.T) {
	for _, word := range []string{"", "hello!", "日本語", "a-b"} {
		if got, ok := englishNeuralPronunciation(word); ok {
			t.Errorf("%q = %q, want no pronunciation", word, got)
		}
	}
}
