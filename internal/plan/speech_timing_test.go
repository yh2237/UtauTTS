package plan

import (
	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/oto"
	"github.com/yh2237/UtauTTS/internal/voicebank"
	"math"
	"testing"
)

func TestBuildUsesTargetPhoneWeightsAndAnchor(t *testing.T) {
	mora := frontend.Mora{Text: "か", Vowel: "a", Phones: []frontend.Phone{
		{Symbol: "k", Role: "onset"}, {Symbol: "a", Role: "nucleus"},
	}}
	selection := voicebank.Selection{
		Position: 0, Mora: mora, Alias: "か", Kind: voicebank.AliasCV,
		Entry: oto.Entry{Filename: "main.wav", Preutterance: 40, Fixed: 80},
	}
	got, err := Build(&voicebank.Bank{Root: "bank"}, "か", []frontend.Mora{mora}, []voicebank.Selection{selection}, Config{
		MoraDurationMS: 100, PhoneWeights: [][]float64{{0.7, 0.3}}, PhoneWeightsSource: "target",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.PhoneTimingSource != "target" || len(got.PhoneTimings) != 2 {
		t.Fatalf("phone timing metadata=%q timings=%v", got.PhoneTimingSource, got.PhoneTimings)
	}
	if math.Abs(got.PhoneTimings[0].DurationMS-70) > 1e-9 || math.Abs(got.PhoneTimings[1].DurationMS-30) > 1e-9 {
		t.Fatalf("phone timings=%v", got.PhoneTimings)
	}
	if math.Abs(got.Units[0].ConsonantMS-110) > 1e-9 {
		t.Fatalf("consonant anchor=%v, want 110", got.Units[0].ConsonantMS)
	}
}
