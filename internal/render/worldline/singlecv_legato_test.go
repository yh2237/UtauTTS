package worldline

import (
	"testing"

	"utautts/internal/frontend"
	"utautts/internal/plan"
)

func legatoPlan() *plan.Plan {
	return &plan.Plan{
		Language: "ja", SingleCV: true,
		Morae: []frontend.Mora{
			{Text: "よ", Consonant: "y", Vowel: "o"}, {Text: "い", Vowel: "i"}, {Text: "、", Pause: true}, {Text: "い", Vowel: "i"}, {Text: "か", Consonant: "k", Vowel: "a"},
		},
	}
}

func TestSingleCVLegatoOnlyForVowelAfterVoicedMora(t *testing.T) {
	synthesisPlan := legatoPlan()
	cases := []struct {
		name string
		unit plan.Unit
		want bool
	}{
		{"vowel after voiced mora", plan.Unit{Role: "mora", Position: 1, AliasKind: "CV"}, true},
		{"vowel after pause", plan.Unit{Role: "mora", Position: 3, AliasKind: "CV"}, false},
		{"consonant mora", plan.Unit{Role: "mora", Position: 4, AliasKind: "CV"}, false},
		{"recorded vowel transition", plan.Unit{Role: "mora", Position: 1, AliasKind: "VCV"}, false},
		{"silent unit", plan.Unit{Role: "mora", Position: 1, AliasKind: "CV", Silent: true}, false},
	}
	for _, c := range cases {
		if got := singleCVLegato(synthesisPlan, c.unit); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	synthesisPlan.SingleCV = false
	if singleCVLegato(synthesisPlan, cases[0].unit) {
		t.Error("continuous-voicebank plan used the single-CV legato join")
	}
}

func TestSingleCVLegatoAnchorsSkipTheVowelOnset(t *testing.T) {
	anchors := singleCVLegatoAnchors(100, 60, 200, 900)
	if len(anchors) != 3 {
		t.Fatalf("anchors = %+v", anchors)
	}
	if anchors[0].TargetMS != 0 || anchors[0].SourceMS != 100+singleCVLegatoSkipMS {
		t.Fatalf("unit head does not start in the stable vowel: %+v", anchors[0])
	}
	for i := 1; i < len(anchors); i++ {
		if anchors[i].SourceMS <= anchors[i-1].SourceMS || anchors[i].TargetMS <= anchors[i-1].TargetMS {
			t.Fatalf("anchors are not increasing: %+v", anchors)
		}
	}
	if anchors[2].SourceMS > 900 || anchors[2].TargetMS != 200 {
		t.Fatalf("last anchor = %+v", anchors[2])
	}
	if singleCVLegatoAnchors(100, 60, 200, 150) != nil {
		t.Fatal("anchors were returned for a source too short to skip the onset")
	}
}
