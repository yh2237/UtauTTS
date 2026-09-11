package render

import (
	"math"
	"testing"

	"utautts/internal/frontend"
	"utautts/internal/plan"
	"utautts/internal/voicebank"
)

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
	if got.overlapMS < got.preutteranceMS-13 || math.Abs(got.preutteranceMS-got.overlapMS-12) > 1 {
		t.Fatalf("onset fade was not shortened: %+v", got)
	}
	minimumTail := math.Max(singleCVMinimumVowelTailMS, unit.DurationMS*singleCVVowelTailRatio)
	if got.preutteranceMS+unit.DurationMS+20-got.consonantMS < minimumTail-1e-9 {
		t.Fatalf("vowel tail was not preserved: %+v", got)
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

func TestSingleCVWorldOverlapKeepsOnsetFadeShort(t *testing.T) {
	p := &plan.Plan{SingleCV: true, Morae: []frontend.Mora{{Consonant: "k", Vowel: "a"}}}
	unit := plan.Unit{Position: 0, PreutteranceMS: 126, OverlapMS: 80}
	if got := singleCVWorldOverlapMS(p, unit, 84); got != 12 {
		t.Fatalf("world overlap = %.3f, want 12", got)
	}
	unit.OverlapMS = 0
	if got := singleCVWorldOverlapMS(p, unit, 84); got != 0 {
		t.Fatalf("zero overlap changed to %.3f", got)
	}
}
