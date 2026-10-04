package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"utautts/internal/prosody"
)

func TestPythonRidgeParityAndModelLoad(t *testing.T) {
	root := "../../../out/training-cleanup/"
	rows, e := readRows(root + "mandarin-fixture.jsonl")
	if e != nil {
		t.Skip("optional Python parity fixture absent")
	}
	raw, e := os.ReadFile(root + "mandarin-py-fit.json")
	if e != nil {
		t.Skip("Python fit absent")
	}
	var py struct {
		Lambda  float64        `json:"lambda"`
		Weights [][]float64    `json:"weights"`
		Metrics map[string]any `json:"metrics"`
	}
	if e = json.Unmarshal(raw, &py); e != nil {
		t.Fatal(e)
	}
	model, e := train(rows)
	if e != nil {
		t.Fatal(e)
	}
	training := model["training"].(map[string]any)
	if training["ridge_lambda"].(float64) != py.Lambda {
		t.Fatal("selected ridge lambda differs")
	}
	weights := model["mandarin_intonation"].(map[string]any)["weights"].([][]float64)
	maximum := 0.0
	for i := range weights {
		for j, v := range weights[i] {
			maximum = math.Max(maximum, math.Abs(v-py.Weights[i][j]))
		}
	}
	if maximum > 1e-9 {
		t.Fatalf("maximum coefficient difference %.12g", maximum)
	}
	got := training["validation"].(map[string]any)
	for _, key := range []string{"mae_cents", "rule_mae_cents"} {
		if got[key] != py.Metrics[key] {
			t.Fatalf("%s differs: %v vs %v", key, got[key], py.Metrics[key])
		}
	}
	loaded, e := prosody.LoadModel(root + "mandarin-go.json")
	if e != nil {
		t.Fatal(e)
	}
	if loaded.MandarinIntonation == nil || !loaded.SupportsLanguage("zh") {
		t.Fatal("Mandarin model did not load")
	}
	values := loaded.MandarinIntonation.MandarinCorrection(map[string]float64{"bias": 1, "tone_1": 1})
	if len(values) != len(knots) {
		t.Fatal("correction knot count differs")
	}
}

func TestRealAISHELL3ObservationParity(t *testing.T) {
	root := "../../../out/training-cleanup/"
	goRows, err := readRows(root + "mandarin-one-go.jsonl")
	if err != nil {
		t.Skip("optional Go AISHELL-3 observations absent")
	}
	pyRows, err := readRows(root + "mandarin-one-py.jsonl")
	if err != nil {
		t.Skip("optional Python AISHELL-3 observations absent")
	}
	if len(goRows) != 3 || len(pyRows) != 3 {
		t.Fatalf("rows: Go %d, Python %d", len(goRows), len(pyRows))
	}
	for i, got := range goRows {
		want := pyRows[i]
		if got.Speaker != want.Speaker || got.Utterance != want.Utterance || len(got.X) != len(want.X) {
			t.Fatalf("utterance %d differs", i)
		}
		for j := range got.X {
			for k := range got.X[j] {
				if math.Abs(got.X[j][k]-want.X[j][k]) > 1e-9 {
					t.Fatalf("feature %d/%d/%d differs", i, j, k)
				}
			}
			for k := range got.Valid[j] {
				if got.Valid[j][k] != want.Valid[j][k] || math.Abs(got.Residual[j][k]-want.Residual[j][k]) > 1e-9 || math.Abs(got.Rule[j][k]-want.Rule[j][k]) > 1e-9 || math.Abs(got.LogF0[j][k]-want.LogF0[j][k]) > 1e-9 {
					t.Fatalf("observation %d/%d/%d differs", i, j, k)
				}
			}
		}
	}
}

func TestCollectionMath(t *testing.T) {
	if got := sampledF0([]float64{0, 100, 200, 800}, 0, .04, .5); got != 150 {
		t.Fatalf("sampled F0 = %g", got)
	}
	x := toneFeatures([]int{1, 2, 3}, 1, []bool{true, false, false}, []bool{false, false, true})
	if len(x) != 20 || x[6] != 1 || x[10] != 1 || x[17] != 1 {
		t.Fatalf("tone feature indices differ: %v", x)
	}
}

func TestTextgridWordsTier(t *testing.T) {
	path := filepath.Join(t.TempDir(), "one.TextGrid")
	raw := "name = \"words\"\nintervals [1]:\n xmin = 0\n xmax = 0.1\n text = \"\"\nintervals [2]:\n xmin = 0.1\n xmax = 0.3\n text = \"ma1\"\nitem [2]:\nintervals [1]:\n xmin = 0\n xmax = 1\n text = \"ignore\"\n"
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := textgridSyllables(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].label != "ma1" || got[0].start != .1 || got[0].end != .3 {
		t.Fatalf("words tier: %+v", got)
	}
}

func TestFullAISHELL3ExportLoads(t *testing.T) {
	path := "../../../out/training-cleanup/mandarin-full-go.json"
	if _, err := os.Stat(path); err != nil {
		t.Skip("optional full AISHELL-3 export absent")
	}
	loaded, err := prosody.LoadModel(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.MandarinIntonation == nil || !loaded.SupportsLanguage("zh") {
		t.Fatal("full Mandarin export does not load")
	}
	if values := loaded.MandarinIntonation.MandarinCorrection(map[string]float64{"bias": 1, "tone_3": 1}); len(values) != 5 {
		t.Fatalf("correction has %d knots", len(values))
	}
}
