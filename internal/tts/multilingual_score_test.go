package tts

import (
	"math"
	"testing"
	"utautts/internal/frontend"
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

func TestSpeechPreviewUsesCanonicalClockAndManualOverride(t *testing.T) {
	var total float64
	for _, phone := range []string{"en-delta", "en-vccv", "en-arpasing", "en-cv"} {
		cfg := Config{Language: "en", Phonemizer: phone, Reading: "HH AH0 L OW1 | W ER1 L D"}
		preview, err := PredictProsody(cfg)
		if err != nil {
			t.Fatal(err)
		}
		sum := 0.0
		for _, d := range preview.MoraDurationsMS {
			sum += d
		}
		if total == 0 {
			total = sum
		} else if math.Abs(total-sum) > .001 {
			t.Fatal(phone, sum, total)
		}
		cfg.MoraDurationsMS = []float64{250}
		manual, err := PredictProsody(cfg)
		if err != nil || manual.MoraDurationsMS[0] != 250 {
			t.Fatal(manual, err)
		}
	}
}
