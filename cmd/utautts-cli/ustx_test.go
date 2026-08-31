package main

import (
	"testing"

	"utautts/internal/plan"
	"utautts/internal/tts"
)

// TestUstxProjectFromSynthesisCarriesPlanTiming makes sure the USTX export
// restores the actual synthesized mora timing from the plan (prosody
// predictions override the uniform mora duration) instead of falling back to
// the fixed MoraDurationMS grid.
func TestUstxProjectFromSynthesisCarriesPlanTiming(t *testing.T) {
	p := &plan.Plan{
		Version: 1,
		Reading: "カキ",
		Units: []plan.Unit{
			{Position: 0, Mora: "カ", NoteStartMS: 0, DurationMS: 100, PitchFactor: 1},
			{Position: 1, Role: "transition", NoteStartMS: 55, DurationMS: 40, PitchFactor: 1},
			{Position: 1, Mora: "キ", NoteStartMS: 130, DurationMS: 120, PitchFactor: 1},
		},
	}
	cfg := tts.Config{Text: "カキ", MoraDurationMS: 140, PauseDurationMS: 180}

	project := ustxProjectFromSynthesis(cfg, p, "bank")
	utterance := project.Utterances[0]
	if len(utterance.AnalysisCache.Morae) != 2 {
		t.Fatalf("morae = %d, want 2", len(utterance.AnalysisCache.Morae))
	}
	wantDurations := []float64{100, 120}
	for index, want := range wantDurations {
		if got := utterance.AutomaticMoraDurMS[index]; got != want {
			t.Fatalf("AutomaticMoraDurMS[%d] = %v, want %v", index, got, want)
		}
	}
	wantPositions := []float64{0, 130}
	for index, want := range wantPositions {
		if got := utterance.AutomaticMoraPosMS[index]; got != want {
			t.Fatalf("AutomaticMoraPosMS[%d] = %v, want %v", index, got, want)
		}
	}
}
