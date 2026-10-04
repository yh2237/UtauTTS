package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"utautts/internal/prosody"
)

func TestPythonFixtureParityAndLoader(t *testing.T) {
	root := "../../../out/training-cleanup/"
	rows, _, e := load(root + "multilingual-fixture.jsonl")
	if e != nil {
		t.Skip("optional Python parity fixture absent")
	}
	model, e := train(rows, "en", "fixture", 10)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile(root + "multilingual-py.json")
	if e != nil {
		t.Skip("Python result absent")
	}
	var py map[string]any
	if e = json.Unmarshal(raw, &py); e != nil {
		t.Fatal(e)
	}
	for _, head := range []string{"duration_log_ratio", "energy_log_ratio"} {
		want := py[head].(map[string]any)
		got := model[head].(map[string]float64)
		if len(want) != len(got) {
			t.Fatalf("%s feature count differs", head)
		}
		for key, v := range want {
			if math.Abs(got[key]-v.(float64)) > 1e-10 {
				t.Fatalf("%s %s: %.12g vs %.12g", head, key, got[key], v)
			}
		}
	}
	for _, split := range []string{"validation", "test"} {
		got := model["evaluation"].(map[string]any)[split].(map[string]any)
		want := py["evaluation"].(map[string]any)[split].(map[string]any)
		for _, key := range []string{"baseline_duration_mae_ms", "model_duration_mae_ms", "pitch_mae_cents", "energy_log_mae"} {
			if math.Abs(got[key].(float64)-want[key].(float64)) > 1e-10 {
				t.Fatalf("%s %s differs", split, key)
			}
		}
	}
	path := filepath.Join("../../../out/training-cleanup", "multilingual-go.json")
	loaded, e := prosody.LoadSpeechModel(path)
	if e != nil {
		t.Fatal(e)
	}
	if loaded.ID != "fixture" || len(loaded.Duration) == 0 || len(loaded.Pitch) != 3 {
		t.Fatal("incomplete loaded model")
	}
}

func TestRejectLeakageAndGenerated(t *testing.T) {
	rows := []record{
		{Version: 1, FeatureVersion: 1, ID: "a", Language: "en", Speaker: "same", Split: "train", Kind: "natural", Alignment: "manual", Corpus: "fixture", License: "test", AudioSHA: "a", Text: "a", Phones: []phone{{Symbol: "aa", Features: []string{"bias"}, Baseline: 100, Duration: 120}}},
		{Version: 1, FeatureVersion: 1, ID: "b", Language: "en", Speaker: "same", Split: "validation", Kind: "natural", Alignment: "manual", Corpus: "fixture", License: "test", AudioSHA: "b", Text: "b", Phones: []phone{{Symbol: "aa", Features: []string{"bias"}, Baseline: 100, Duration: 120}}},
	}
	if e := validate(rows, "en"); e == nil {
		t.Fatal("speaker leakage accepted")
	}
	rows[1].Speaker = "other"
	rows[0].Kind = "generated"
	if e := validate(rows, "en"); e == nil {
		t.Fatal("generated label accepted")
	}
}
