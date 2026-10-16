package tts

import (
	"github.com/yh2237/UtauTTS/internal/frontend"
	"testing"
)

func TestMandarinNeutralToneShorterWithAudibleNasal(t *testing.T) {
	_, full, _ := frontend.ParseChineseCVVC("", "jin1", nil)
	_, neutral, _ := frontend.ParseChineseCVVC("", "jin5", nil)
	a, b := speechPhoneDurations(full, 120)[0], speechPhoneDurations(neutral, 120)[0]
	if len(a) != 3 || a[2] <= 0 {
		t.Fatal(a)
	}
	for i := range a {
		if b[i] >= a[i] || b[i] <= 0 {
			t.Fatal(a, b)
		}
	}
}
