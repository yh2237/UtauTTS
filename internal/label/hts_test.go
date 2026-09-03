package label

import (
	"strings"
	"testing"

	"utautts/internal/frontend"
	"utautts/internal/plan"
)

func TestHTSUsesLeadingMarginAndEffectivePreutterance(t *testing.T) {
	p := &plan.Plan{
		Reading: "こんにちは。", LeadingMarginMS: 60, DurationMS: 680,
		Units: []plan.Unit{
			{Position: 0, Role: "mora", Mora: "こ", NoteStartMS: 0, DurationMS: 100, EffectivePreutteranceMS: 60},
			{Position: 1, Role: "mora", Mora: "ん", NoteStartMS: 100, DurationMS: 90},
			{Position: 2, Role: "mora", Mora: "に", NoteStartMS: 190, DurationMS: 100, EffectivePreutteranceMS: 40},
			{Position: 3, Role: "mora", Mora: "ち", NoteStartMS: 290, DurationMS: 100, EffectivePreutteranceMS: 35},
			{Position: 4, Role: "mora", Mora: "は", NoteStartMS: 390, DurationMS: 100, EffectivePreutteranceMS: 45},
		},
	}
	got, err := HTS(p, []float64{100, 90, 100, 100, 100, 180}, 830)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"0 600000 k", "600000 1600000 o", "1600000 2100000 N",
		"2100000 2500000 n", "2500000 3150000 i", "5500000 7300000 pau",
		"7300000 8300000 sil",
	} {
		if !strings.Contains(got, want+"\n") {
			t.Fatalf("label missing %q:\n%s", want, got)
		}
	}
}

func TestHTSUsesAnalyzedEnglishUnits(t *testing.T) {
	p := &plan.Plan{
		Reading: "HH AH L OW", LeadingMarginMS: 20,
		Morae: []frontend.Mora{
			{Text: "hV", Consonant: "h", Vowel: "V"},
			{Text: "loU", Consonant: "l", Vowel: "oU"},
		},
		Units: []plan.Unit{
			{Position: 0, Role: "mora", EffectivePreutteranceMS: 20},
			{Position: 1, Role: "mora", EffectivePreutteranceMS: 20},
		},
	}
	got, err := HTS(p, []float64{100, 100}, 220)
	if err != nil {
		t.Fatal(err)
	}
	for _, phone := range []string{"h", "V", "l", "oU"} {
		if !strings.Contains(got, " "+phone+"\n") {
			t.Fatalf("label missing %q:\n%s", phone, got)
		}
	}
}

func TestHTSRejectsMismatchedMoraDurations(t *testing.T) {
	_, err := HTS(&plan.Plan{Reading: "あ", Units: []plan.Unit{{Position: 0, Role: "mora"}}}, nil, 100)
	if err == nil {
		t.Fatal("expected mora duration mismatch")
	}
}

func TestHTSLabelsLeadingSilenceBeforeVowel(t *testing.T) {
	p := &plan.Plan{
		Reading: "あ", LeadingMarginMS: 40,
		Units: []plan.Unit{{Position: 0, Role: "mora", Mora: "あ"}},
	}
	got, err := HTS(p, []float64{100}, 160)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"0 400000 sil", "400000 1400000 a", "1400000 1600000 sil"} {
		if !strings.Contains(got, want+"\n") {
			t.Fatalf("label missing %q:\n%s", want, got)
		}
	}
}
