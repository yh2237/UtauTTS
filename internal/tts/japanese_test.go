package tts

import "testing"

func TestQuestionRiseOnlyAppliesToFinalPhrase(t *testing.T) {
	for _, tc := range []struct {
		text string
		want bool
	}{
		{"準備はできましたか？それでは出発します。", false},
		{"そうですか？", true}, {"「そうですか？」\n", true},
		{"本当に？！", true}, {"出発します。", false}, {"", false},
	} {
		if got := finalPhraseIsQuestion(tc.text); got != tc.want {
			t.Errorf("%q: got %v want %v", tc.text, got, tc.want)
		}
	}
}
