package plan

import (
	"bytes"
	"encoding/json"
	"testing"

	"utautts/internal/frontend"
	"utautts/internal/oto"
	"utautts/internal/voicebank"
)

func TestBuildPlacesMoraeAndPause(t *testing.T) {
	morae, err := frontend.ParseKana("あ、んっ")
	if err != nil {
		t.Fatal(err)
	}
	bank := &voicebank.Bank{Root: "bank"}
	selections := []voicebank.Selection{
		{
			Position: 0, Mora: morae[0], Alias: "あ", Entry: oto.Entry{Filename: "a.wav"},
			Kind: voicebank.AliasCV, FallbackTier: 1,
			CandidateCount: 3, TargetScore: 100, JoinScore: -2, JoinProbability: 0.75, PathScore: 98,
		},
		{Position: 2, Mora: morae[2], Alias: "ん", Entry: oto.Entry{Filename: "n.wav"}},
		{Position: 3, Mora: morae[3], Alias: "っ", Entry: oto.Entry{Filename: "cl.wav"}},
	}
	got, err := Build(bank, "あ、んっ", morae, selections, Config{
		MoraDurationMS: 100, PauseDurationMS: 200, SelectionMode: voicebank.SelectionGreedy,
		AliasPolicy:  voicebank.AliasPolicyCVOnly,
		JoinCostMode: "learned", JoinModelVersion: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Units[1].NoteStartMS != 300 || got.Units[1].DurationMS != 90 {
		t.Fatalf("nasal unit = %+v", got.Units[1])
	}
	if got.Units[2].NoteStartMS != 390 || got.Units[2].DurationMS != 65 {
		t.Fatalf("closure unit = %+v", got.Units[2])
	}
	if got.DurationMS != 455 {
		t.Fatalf("duration = %v", got.DurationMS)
	}
	if got.Version != Version || got.SelectionMode != "greedy" || got.AliasPolicy != string(voicebank.AliasPolicyCVOnly) || got.JoinCostMode != "learned" || got.JoinModelVersion != 1 {
		t.Fatalf("plan audit = %#v", got)
	}
	if unit := got.Units[0]; unit.AliasKind != string(voicebank.AliasCV) || unit.FallbackTier != 1 || unit.CandidateCount != 3 || unit.TargetScore != 100 || unit.JoinScore != -2 || unit.JoinProbability != 0.75 || unit.PathScore != 98 {
		t.Fatalf("selection score audit = %#v", unit)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoded, []byte(`"fallback_tier":0`)) {
		t.Fatalf("primary fallback tier was omitted from plan JSON: %s", encoded)
	}
}

func TestBuildUsesPerMoraDurationOverride(t *testing.T) {
	morae, err := frontend.ParseKana("あい")
	if err != nil {
		t.Fatal(err)
	}
	bank := &voicebank.Bank{Root: "bank"}
	selections := []voicebank.Selection{
		{Position: 0, Mora: morae[0], Alias: "あ", Entry: oto.Entry{Filename: "a.wav"}},
		{Position: 1, Mora: morae[1], Alias: "い", Entry: oto.Entry{Filename: "i.wav"}},
	}
	got, err := Build(bank, "あい", morae, selections, Config{
		MoraDurationMS: 100, PauseDurationMS: 180,
		MoraDurationsMS: []float64{250, 0},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Units) != 2 || got.Units[0].DurationMS != 250 || got.Units[1].DurationMS != 100 {
		t.Fatalf("units = %+v", got.Units)
	}
	if got.Units[1].NoteStartMS != 250 || got.DurationMS != 350 {
		t.Fatalf("timing = %+v duration=%v", got.Units, got.DurationMS)
	}
}

func TestBuildUsesPerPauseDurationOverride(t *testing.T) {
	morae, err := frontend.ParseKana("あ、い")
	if err != nil {
		t.Fatal(err)
	}
	bank := &voicebank.Bank{Root: "bank"}
	selections := []voicebank.Selection{
		{Position: 0, Mora: morae[0], Alias: "あ", Entry: oto.Entry{Filename: "a.wav"}},
		{Position: 2, Mora: morae[2], Alias: "い", Entry: oto.Entry{Filename: "i.wav"}},
	}
	got, err := Build(bank, "あ、い", morae, selections, Config{
		MoraDurationMS: 100, PauseDurationMS: 180,
		MoraDurationsMS: []float64{0, 320, 0},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Units) != 2 || got.Units[1].NoteStartMS != 420 || got.DurationMS != 520 {
		t.Fatalf("pause override was not applied: units=%+v duration=%v", got.Units, got.DurationMS)
	}
}

func TestBuildAddsCVVCTransitionWithoutChangingMoraTimeline(t *testing.T) {
	morae, err := frontend.ParseKana("あか")
	if err != nil {
		t.Fatal(err)
	}
	bank := &voicebank.Bank{Root: "bank"}
	transition := &voicebank.Selection{
		Position: 1, Mora: morae[1], Alias: "a k", Kind: voicebank.AliasVC,
		Entry: oto.Entry{Filename: "ak.wav", Preutterance: 50, Overlap: 20, Fixed: 20},
	}
	selections := []voicebank.Selection{
		{Position: 0, Mora: morae[0], Alias: "あ", Kind: voicebank.AliasCV, Entry: oto.Entry{Filename: "a.wav"}},
		{Position: 1, Mora: morae[1], Alias: "か", Kind: voicebank.AliasCV, Entry: oto.Entry{Filename: "ka.wav"}, Transition: transition},
	}
	got, err := Build(bank, "あか", morae, selections, Config{MoraDurationMS: 100, PauseDurationMS: 180})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Units) != 3 {
		t.Fatalf("units = %#v", got.Units)
	}
	if got.Units[1].Role != "transition" || got.Units[1].Alias != "a k" || got.Units[1].Position != 1 {
		t.Fatalf("transition unit = %#v", got.Units[1])
	}
	if got.Units[1].PreutteranceMS != 50 || got.Units[1].OverlapMS != 20 || got.Units[1].DurationMS != 30 {
		t.Fatalf("transition timing = %#v", got.Units[1])
	}
	if got.Units[2].Role != "mora" || got.Units[2].NoteStartMS != 100 || got.Units[2].DurationMS != 100 {
		t.Fatalf("main unit = %#v", got.Units[2])
	}
	if got.DurationMS != 200 {
		t.Fatalf("duration = %v", got.DurationMS)
	}
}
