package render

import (
	"math"
	"reflect"
	"testing"

	"utautts/internal/frontend"
	"utautts/internal/plan"
	"utautts/internal/voicebank"
)

func TestOnsetOverlapMSAdjustsByClass(t *testing.T) {
	if got := onsetOverlapMS("k", 100, 60); got != 10 {
		t.Fatalf("stop overlap = %.3f, want 10", got)
	}
	if got := onsetOverlapMS("m", 100, 10); got != 50 {
		t.Fatalf("sonorant overlap = %.3f, want 50", got)
	}
	if got := onsetOverlapMS("s", 100, 30); got != 30 {
		t.Fatalf("fricative overlap = %.3f, want 30", got)
	}
}

func TestNormalizePlanTimingAppliesOnsetOverlap(t *testing.T) {
	p := &plan.Plan{Morae: []frontend.Mora{{Consonant: "k", Vowel: "a"}}}
	unit := plan.Unit{Role: "mora", Position: 0, AliasKind: "CV", DurationMS: 140,
		PreutteranceMS: 100, OverlapMS: 60, ConsonantMS: 120}
	if got := normalizePlanTiming(p, unit, 20); got.overlapMS != 10 {
		t.Fatalf("stop overlap = %.3f, want 10", got.overlapMS)
	}
}

func TestNormalizeSingleCVTimingProtectsOnsetAndVowelTail(t *testing.T) {
	p := &plan.Plan{SingleCV: true, Morae: []frontend.Mora{{Consonant: "k", Vowel: "a"}}}
	unit := plan.Unit{
		Role: "mora", Position: 0, DurationMS: 140, PreutteranceMS: 126,
		ConsonantMS: 184, OverlapMS: 0,
		SpeechProfile: &voicebank.SpeechProfile{TrimmedLengthMS: 266, VowelTailMS: 82},
	}
	got := normalizePlanTiming(p, unit, 20)
	if !got.cvApplied || got.preutteranceMS >= unit.PreutteranceMS || got.consonantMS >= unit.ConsonantMS {
		t.Fatalf("timing was not bounded: %+v", got)
	}
	if got.preutteranceMS > 85 || got.preutteranceMS < 80 {
		t.Fatalf("preutterance = %.3f", got.preutteranceMS)
	}
	if got.overlapMS < got.preutteranceMS-19 || math.Abs(got.preutteranceMS-got.overlapMS-18) > 1 {
		t.Fatalf("onset fade was not shortened: %+v", got)
	}
	minimumTail := math.Max(singleCVMinimumVowelTailMS, unit.DurationMS*singleCVVowelTailRatio)
	if got.preutteranceMS+unit.DurationMS+20-got.consonantMS < minimumTail-1e-9 {
		t.Fatalf("vowel tail was not preserved: %+v", got)
	}
}

func TestNormalizeVCVTimingKeepsValidOTOAnchors(t *testing.T) {
	unit := plan.Unit{
		Role: "mora", AliasKind: "VCV", DurationMS: 140,
		PreutteranceMS: 210, OverlapMS: 70, ConsonantMS: 360,
		SpeechProfile: &voicebank.SpeechProfile{
			Applied: true, TrimmedLengthMS: 560, StableStartMS: 335,
		},
	}
	got := normalizePlanTiming(&plan.Plan{}, unit, 20)
	if got.preutteranceMS != 210 || got.consonantMS != 360 || got.overlapMS != 70 || got.scale != 1 {
		t.Fatalf("valid VCV anchors changed: %+v", got)
	}
	if got.cvApplied {
		t.Fatalf("valid VCV was reported as corrected: %+v", got)
	}
}

func TestNormalizeVCVTimingRepairsBrokenBoundaries(t *testing.T) {
	cases := []plan.Unit{
		{Role: "mora", AliasKind: "VCV", DurationMS: 140, PreutteranceMS: 210, OverlapMS: 260, ConsonantMS: 360},
		{Role: "mora", AliasKind: "VCV", DurationMS: 140, PreutteranceMS: 210, OverlapMS: 70, ConsonantMS: 120},
		{Role: "mora", AliasKind: "VCV", DurationMS: 140, PreutteranceMS: -10, OverlapMS: 0, ConsonantMS: 360},
		{Role: "mora", AliasKind: "VCV", DurationMS: 140, PreutteranceMS: 620, OverlapMS: 70, ConsonantMS: 700,
			SpeechProfile: &voicebank.SpeechProfile{Applied: true, TrimmedLengthMS: 560}},
	}
	for _, unit := range cases {
		got := normalizePlanTiming(&plan.Plan{}, unit, 20)
		if !got.cvApplied {
			t.Fatalf("broken VCV was not corrected: %+v", unit)
		}
		if got.overlapMS > got.preutteranceMS || got.consonantMS < got.preutteranceMS {
			t.Fatalf("broken VCV was not repaired: %+v", got)
		}
	}
}

func TestNormalizedPhoneTimingUnitsKeepValidVCVAnchors(t *testing.T) {
	p := &plan.Plan{Units: []plan.Unit{{Role: "mora", AliasKind: "VCV", DurationMS: 140,
		PreutteranceMS: 210, OverlapMS: 70, ConsonantMS: 360}}}
	units := normalizedPhoneTimingUnits(p, 20)
	if len(units) != 1 || units[0].PreutteranceMS != 210 || units[0].ConsonantMS != 360 {
		t.Fatalf("normalized phone units = %+v", units)
	}
	if p.Units[0].PreutteranceMS != 210 || p.Units[0].ConsonantMS != 360 {
		t.Fatalf("source plan was mutated: %+v", p.Units[0])
	}
}

func TestWorldlineVCVTimingKeepsValidOTOAnchors(t *testing.T) {
	unit := plan.Unit{Role: "mora", AliasKind: "VCV", DurationMS: 140,
		PreutteranceMS: 210, OverlapMS: 70, ConsonantMS: 360,
		SpeechProfile: &voicebank.SpeechProfile{Applied: true, TrimmedLengthMS: 560, StableStartMS: 335}}
	plain := worldlineTiming(&plan.Plan{}, unit, 20)
	if plain.preutteranceMS != 210 || plain.consonantMS != 360 || plain.overlapMS != 70 {
		t.Fatalf("WORLD changed valid oto anchors: %+v", plain)
	}
	broken := unit
	broken.OverlapMS = 400
	got := worldlineTiming(&plan.Plan{}, broken, 20)
	if !got.cvApplied || got.overlapMS > got.preutteranceMS {
		t.Fatalf("WORLD did not repair a broken VCV boundary: %+v", got)
	}
}

func TestWorldlinePhoneTimingUnitsDoesNotMutatePlan(t *testing.T) {
	unit := plan.Unit{Role: "mora", AliasKind: "VCV", DurationMS: 140,
		PreutteranceMS: 210, OverlapMS: 70, ConsonantMS: 360}
	p := &plan.Plan{Units: []plan.Unit{unit}}
	got := worldlinePhoneTimingUnits(p, 20)
	if len(got) != 1 || !reflect.DeepEqual(got[0], unit) {
		t.Fatalf("WORLD default phone timing changed: %+v", got)
	}
	if !reflect.DeepEqual(p.Units[0], unit) {
		t.Fatalf("source plan was mutated: %+v", p.Units[0])
	}
}

func TestSingleCVBoundaryEligibilityProtectsStops(t *testing.T) {
	p := &plan.Plan{SingleCV: true, Morae: []frontend.Mora{
		{Consonant: "a", Vowel: "a"},
		{Consonant: "s", Vowel: "i"},
	}}
	previous := renderedUnit{index: 0, unit: plan.Unit{Role: "mora", Position: 0}}
	current := renderedUnit{index: 1, unit: plan.Unit{Role: "mora", Position: 1}}
	if !singleCVBoundaryEligible(p, previous, current) || singleCVProtectedOnset(p, current.unit) {
		t.Fatal("fricative boundary was rejected")
	}
	p.Morae[1].Consonant = "k"
	if !singleCVProtectedOnset(p, current.unit) {
		t.Fatal("stop boundary was not protected")
	}
}

func TestSingleCVWorldOverlapKeepsOnsetFadeControlled(t *testing.T) {
	p := &plan.Plan{SingleCV: true, Morae: []frontend.Mora{{Consonant: "k", Vowel: "a"}}}
	unit := plan.Unit{Position: 0, PreutteranceMS: 126, OverlapMS: 80}
	if got := singleCVWorldOverlapMS(p, unit, 84); got != 18 {
		t.Fatalf("world overlap = %.3f, want 18", got)
	}
	unit.OverlapMS = 0
	if got := singleCVWorldOverlapMS(p, unit, 84); got != 0 {
		t.Fatalf("zero overlap changed to %.3f", got)
	}
}

func TestSingleCVWorldOverlapAddsVowelBoundaryBlend(t *testing.T) {
	p := &plan.Plan{SingleCV: true, Morae: []frontend.Mora{
		{Vowel: "a"}, {Vowel: "a"}, {Vowel: "i"}, {Consonant: "k", Vowel: "a"},
	}}
	if got := singleCVWorldOverlapMS(p, plan.Unit{Position: 1}, 0); got != singleCVSameVowelOverlapMS {
		t.Fatalf("same-vowel overlap = %.3f, want %.3f", got, singleCVSameVowelOverlapMS)
	}
	if got := singleCVWorldOverlapMS(p, plan.Unit{Position: 2}, 0); got != singleCVDefaultVowelOverlapMS {
		t.Fatalf("vowel overlap = %.3f, want %.3f", got, singleCVDefaultVowelOverlapMS)
	}
	if got := singleCVWorldOverlapMS(p, plan.Unit{Position: 3}, 0); got != 0 {
		t.Fatalf("consonant onset overlap = %.3f, want 0", got)
	}
}
