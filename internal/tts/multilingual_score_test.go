package tts

import (
	"github.com/yh2237/UtauTTS/internal/frontend"
	"math"
	"testing"
)

func TestSpeechClockIndependentOfEnglishBankFormat(t *testing.T) {
	parsers := []func(string, string, map[string]string) (string, []frontend.Mora, error){frontend.ParseEnglishDelta, frontend.ParseEnglishVCCV, frontend.ParseEnglishARPAsing, frontend.ParseEnglishCV}
	var reference []float64
	for i, parse := range parsers {
		_, morae, err := parse("", "HH AH0 L OW1 | W ER1 L D", nil)
		if err != nil {
			t.Fatal(err)
		}
		var flat []float64
		for _, spans := range speechPhoneDurations(morae, 120) {
			flat = append(flat, spans...)
		}
		if i == 0 {
			reference = flat
			continue
		}
		if len(flat) != len(reference) {
			t.Fatalf("format %d: %v versus %v", i, flat, reference)
		}
		for j := range flat {
			if math.Abs(flat[j]-reference[j]) > .001 {
				t.Fatalf("format %d phone %d: %v versus %v", i, j, flat, reference)
			}
		}
	}
}

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
