package main

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/yh2237/UtauTTS/internal/linguistics"
	"github.com/yh2237/UtauTTS/internal/prosody"
	"github.com/yh2237/gograd/tensor"
)

func TestExportLoadsAndPredicts(t *testing.T) {
	m, e := newTCN(2, 3, []int{1, 2}, tensor.CPU, 1)
	if e != nil {
		t.Fatal(e)
	}
	c := config{Output: "out/test.json", ID: "go-test", Name: "Go test", Language: "ja", Corpus: "test", License: "MIT License", Notices: []string{"licenses/TSUKUYOMI-CORPUS.txt"}, Hidden: 3, Dilations: []int{1, 2}, Low: -250, High: 250, Frame: 10, RenderStrength: .32, RenderSmoothing: 20, RenderP99: 75, RenderMax: 90}
	payload := export(m, []string{"bias", "mora_progress"}, c, "abc", nil, nil, nil, nil, nil, 0, 0, 0, 0, 0, nil)
	data, e := json.Marshal(payload)
	if e != nil {
		t.Fatal(e)
	}
	model, e := prosody.ParseModel(data)
	if e != nil {
		t.Fatal(e)
	}
	curve := model.PredictFrameContour([]linguistics.Mora{{Text: "あ", Vowel: "a"}}, nil, []prosody.MoraTiming{{StartMS: 0, DurationMS: 100}}, 100, false)
	if curve == nil || len(curve.Cents) == 0 {
		t.Fatalf("no contour: %#v", curve)
	}
	for _, v := range curve.Cents {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Fatalf("nonfinite contour")
		}
	}
}
