package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/yh2237/gograd/tensor"
	"utautts/internal/frontend"
	"utautts/internal/prosody"
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
	curve := model.PredictFrameContour([]frontend.Mora{{Text: "あ", Vowel: "a"}}, nil, []prosody.MoraTiming{{StartMS: 0, DurationMS: 100}}, 100, false)
	if curve == nil || len(curve.Cents) == 0 {
		t.Fatalf("no contour: %#v", curve)
	}
	for _, v := range curve.Cents {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Fatalf("nonfinite contour")
		}
	}
}

func TestWorldHarvestParity(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows WORLD DLL")
	}
	root := filepath.Join("..", "..", "..")
	dll := filepath.Join(root, "runtime", "utautts-world-engine.dll")
	if _, e := os.Stat(dll); e != nil {
		t.Skip(e)
	}
	rows, e := loadRecords(filepath.Join(root, "out", "english-frame-v1", "corpus.jsonl"))
	if e != nil {
		t.Skip(e)
	}
	f0, e := worldF0(rows[0], 10, dll)
	if e != nil {
		t.Fatal(e)
	}
	if len(f0) != 773 {
		t.Fatalf("WORLD frames=%d want 773", len(f0))
	}
	for i, want := range map[int]float64{0: 0, 1: 0, 2: 0, 10: 0, 20: 155.6063113, 30: 158.52098054, 40: 165.66818625, 100: 164.47934528} {
		if math.Abs(f0[i]-want) > 1e-5 {
			t.Errorf("WORLD frame %d: %.8f want %.8f", i, f0[i], want)
		}
	}
}
