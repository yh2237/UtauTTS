package main

import (
	"math/rand"
	"path/filepath"
	"testing"
	"time"

	"utautts/internal/frontend"
	"utautts/internal/render"
	"utautts/internal/tts"
)

func TestNextPairDoesNotRepeatUninformativeVote(t *testing.T) {
	item := pairOption{prompt: prompt{ID: "one"}, left: candidate{ID: "baseline"}, right: candidate{ID: "other"}}
	s := &server{pairs: []pairOption{item}, random: rand.New(rand.NewSource(1))}
	_, _, _, ok := s.nextPair()
	if !ok {
		t.Fatal("unused pair was skipped")
	}
	s.votes = []vote{{PromptID: "one", Left: "baseline", Right: "other", Choice: "tie"}}
	_, _, _, ok = s.nextPair()
	if ok {
		t.Fatal("tie was offered again")
	}
}

func TestNextPairSkipsWholePromptWhenRequested(t *testing.T) {
	one := pairOption{prompt: prompt{ID: "one"}, left: candidate{ID: "baseline"}, right: candidate{ID: "other"}}
	two := pairOption{prompt: prompt{ID: "one"}, left: candidate{ID: "baseline"}, right: candidate{ID: "third"}}
	s := &server{pairs: []pairOption{one, two}, votes: []vote{{PromptID: "one", Left: "baseline", Right: "other", Choice: "skip_prompt"}}, random: rand.New(rand.NewSource(1))}
	_, _, _, ok := s.nextPair()
	if ok {
		t.Fatal("skipped prompt still has a pair")
	}
}

func TestWriteManifestRejectsChangedModelOrCandidates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	original := manifest{Version: sessionVersion, Mode: "contour", CreatedAt: time.Now(),
		ModelFile: "model.json", ModelHash: "first", Candidates: makeContourCandidates()}
	if err := writeManifest(path, original); err != nil {
		t.Fatal(err)
	}
	restarted := original
	restarted.CreatedAt = restarted.CreatedAt.Add(time.Hour)
	if err := writeManifest(path, restarted); err != nil {
		t.Fatalf("same session should resume: %v", err)
	}
	restarted.ModelHash = "second"
	if err := writeManifest(path, restarted); err == nil {
		t.Fatal("changed model reused existing votes")
	}
}

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

func TestRankCandidatesIgnoresInconclusiveVotes(t *testing.T) {
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
	if ranking[2].ID != "strength-1_8" || ranking[2].Score != 0 || ranking[2].Comparisons != 1 {
		t.Fatalf("last ranking = %#v", ranking[2])
	}
}

func TestPairKeyIgnoresSideOrder(t *testing.T) {
	if pairKey("prompt", "b", "a") != pairKey("prompt", "a", "b") {
		t.Fatal("pair key changed with side order")
	}
}

func TestDistinctPitchFramesIgnoresShortOrUnvoicedDifference(t *testing.T) {
	left := &render.PitchCurve{FrameMS: 10, Cents: make([]float64, 40)}
	right := &render.PitchCurve{FrameMS: 10, Cents: make([]float64, 40)}
	active := make([]bool, 40)
	for index := 0; index < 40; index++ {
		active[index] = index < 30
		right.Cents[index] = 28
	}
	if got := distinctPitchFrames(left, right, active, 25); got != 30 {
		t.Fatalf("distinct frames = %d, want 30", got)
	}
	if got := distinctPitchFrames(left, right, active[30:], 25); got != 0 {
		t.Fatalf("inactive frames = %d, want 0", got)
	}
}

func TestBuildStrengthPairsPreservesAllOptions(t *testing.T) {
	options, err := buildPairs(manifest{Mode: "strength", Prompts: []prompt{{ID: "one"}},
		Candidates: makeCandidates([]float64{1, 1.2, 1.4})}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(options) != 3 {
		t.Fatalf("pair count = %d, want 3", len(options))
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
