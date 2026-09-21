package main

import (
	"testing"

	"utautts/internal/frontend"
	"utautts/internal/render"
	"utautts/internal/tts"
)

func TestParseStrengthsSortsAndDeduplicates(t *testing.T) {
	got, err := parseStrengths("1.6, 1.0,1.6,1.2")
	if err != nil {
		t.Fatal(err)
	}
	want := []float64{1, 1.2, 1.6}
	if len(got) != len(want) {
		t.Fatalf("strength count = %d, want %d", len(got), len(want))
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("strength[%d] = %v, want %v", index, got[index], want[index])
		}
	}
}

func TestRankCandidatesCountsWinsAndTies(t *testing.T) {
	candidates := makeCandidates([]float64{1, 1.4, 1.8})
	votes := []vote{
		{Left: "strength-1", Right: "strength-1_4", Choice: "right"},
		{Left: "strength-1_4", Right: "strength-1_8", Choice: "left"},
		{Left: "strength-1", Right: "strength-1_8", Choice: "tie"},
		{Left: "strength-1", Right: "strength-1_4", Choice: "neither"},
	}
	ranking := rankCandidates(candidates, votes)
	if ranking[0].ID != "strength-1_4" || ranking[0].Score != 1 || ranking[0].Comparisons != 2 {
		t.Fatalf("first ranking = %#v", ranking[0])
	}
	if ranking[2].ID != "strength-1_8" || ranking[2].Score != .25 {
		t.Fatalf("last ranking = %#v", ranking[2])
	}
}

func TestPairKeyIgnoresSideOrder(t *testing.T) {
	if pairKey("prompt", "b", "a") != pairKey("prompt", "a", "b") {
		t.Fatal("pair key changed with side order")
	}
}

func TestTransformContourKeepsPauseFramesSilent(t *testing.T) {
	preview := &tts.ProsodyPreview{
		Morae:           []frontend.Mora{{Text: "ア"}, {Pause: true}, {Text: "イ"}},
		MoraDurationsMS: []float64{100, 100, 100},
		MoraPositionsMS: []float64{50, 150, 250},
		FramePitchCurve: &render.PitchCurve{FrameMS: 10, Cents: make([]float64, 30)},
	}
	for index := range preview.FramePitchCurve.Cents {
		preview.FramePitchCurve.Cents[index] = float64(index)
	}
	curve, err := transformContour(preview, "テスト", candidate{Strength: 1, OffsetCents: 10})
	if err != nil {
		t.Fatal(err)
	}
	if curve.Cents[0] != 10 || curve.Cents[9] != 19 || curve.Cents[20] != 30 || curve.Cents[29] != 39 {
		t.Fatalf("sounding frames = %#v", curve.Cents)
	}
	for index := 10; index < 20; index++ {
		if curve.Cents[index] != 0 {
			t.Fatalf("pause frame %d = %v, want 0", index, curve.Cents[index])
		}
	}
}
