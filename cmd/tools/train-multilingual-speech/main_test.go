package main

import (
	"testing"
)

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
