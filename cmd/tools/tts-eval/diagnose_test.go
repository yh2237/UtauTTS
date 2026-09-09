package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"utautts/internal/render"
)

func TestValidatePrompts(t *testing.T) {
	for _, rows := range [][]prompt{
		{{ID: "x", Text: "hello"}, {ID: "x", Text: "hello"}},
		{{ID: "x"}},
		{{ID: "x", Text: "hello", Language: "zh", Phonemizer: "en-vccv"}},
		{{ID: "x", Text: "hello", MoraDurationsMS: []float64{-1}}},
		{{ID: "x", Text: "hello", PitchCurve: &render.PitchCurve{FrameMS: 0, Cents: []float64{0}}}},
	} {
		if validatePrompts(rows) == nil {
			t.Fatal("invalid corpus accepted")
		}
	}
	if err := validatePrompts([]prompt{{ID: "legacy", Text: "こんにちは"}, {ID: "reading", Reading: "ma1", Language: "zh"}}); err != nil {
		t.Fatal(err)
	}
}

func TestDiagnosticsPreserveFrontendWhenCandidatesMissing(t *testing.T) {
	bank := t.TempDir()
	if err := os.WriteFile(filepath.Join(bank, "oto.ini"), []byte("missing.wav=a,0,0,0,0,0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "result")
	err := diagnoseCorpus(bank, out, []prompt{{ID: "first", Language: "en", Reading: "HH AH0"}, {ID: "second", Language: "zh", Reading: "ma3"}})
	if err == nil {
		t.Fatal("expected missing candidates")
	}
	data, err := os.ReadFile(filepath.Join(out, "diagnostics.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []diagnostic
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows: %d", len(rows))
	}
	for _, row := range rows {
		if len(row.Units) == 0 || row.Stage != "candidate_selection" || row.Error == "" {
			t.Fatalf("lost diagnosis: %+v", row)
		}
	}
	if err = diagnoseCorpus(bank, out, nil); err == nil {
		t.Fatal("existing output accepted")
	}
}
