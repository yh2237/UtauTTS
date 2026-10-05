package main

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/yh2237/gograd/tensor"
)

func TestAllDataTrainingSplit(t *testing.T) {
	rows := []record{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}, {ID: "e"}}
	train, valid, test := splitForTraining(rows, true, true)
	if len(train) != len(rows) || len(test) != 0 || len(valid) == 0 {
		t.Fatalf("train=%d validation=%d test=%d", len(train), len(valid), len(test))
	}
	for i, r := range train {
		if r.ID != rows[i].ID {
			t.Fatalf("training order %v", ids(train))
		}
	}
	seen := map[string]bool{}
	for _, r := range train {
		seen[r.ID] = true
	}
	for _, r := range valid {
		if !seen[r.ID] {
			t.Fatalf("validation %s not included in training", r.ID)
		}
	}
}

func TestAudioRootAndExportOptions(t *testing.T) {
	root := t.TempDir()
	audio := filepath.Join(root, "voice.wav")
	if err := os.WriteFile(audio, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := resolveAudioPath("voice.wav", "", root); got != audio {
		t.Fatalf("resolved %q want %q", got, audio)
	}
	m, err := newTCN(1, 2, []int{1}, tensor.CPU, 1)
	if err != nil {
		t.Fatal(err)
	}
	c := config{Output: "out/test.json", Language: "ja", Corpus: "fixture", License: "MIT", Notices: []string{"license"}, Hidden: 2, Dilations: []int{1}, Low: -250, High: 250, Frame: 10, Description: "fixture model", RecommendedRenderers: []string{"renderer-a", "renderer-b"}}
	payload := export(m, []string{"bias"}, c, "hash", nil, nil, nil, nil, nil, 0, 0, 0, 0, 0, nil)
	if payload["description"] != "fixture model" || len(payload["recommended_renderers"].([]string)) != 2 {
		t.Fatalf("export options: %v", payload)
	}
}

func TestEnglishFlatBaseline(t *testing.T) {
	c := config{Low: -400, High: 400, Frame: 10, RenderStrength: .65, RenderSmoothing: 20, RenderP99: 200, RenderMax: 250}
	ex := example{Targets: []float32{.25, -.5, .125}, Mask: []bool{true, true, false}}
	got := flatBaselineMetrics([]example{ex}, c)
	if got["raw_mae_cents"] != 150 {
		t.Fatalf("raw baseline=%v", got)
	}
	reference := renderContour([]float64{100, -200, 50}, ex.Mask, 10, .65, 20, 200, 250, -400, 400)
	want := (math.Abs(reference[0]) + math.Abs(reference[1])) / 2
	if math.Abs(got["rendered_mae_cents"]-want) > 1e-9 {
		t.Fatalf("rendered baseline=%v want %v", got, want)
	}
}

func TestPredictCorpusJSONAndJSONL(t *testing.T) {
	m, err := newTCN(2, 3, []int{1}, tensor.CPU, 1)
	if err != nil {
		t.Fatal(err)
	}
	c := config{Language: "ja", Frame: 10, Low: -250, High: 250}
	fixture := `{"id":"sample","text":"あい","reading":"アイ","tokens":[{"mora":"ア","vowel":"a","start_ms":0,"end_ms":100,"accent_phrase_length":2},{"mora":"イ","vowel":"i","start_ms":100,"end_ms":200,"accent_phrase_length":2}]}`
	for _, ext := range []string{".jsonl", ".json"} {
		path := filepath.Join(t.TempDir(), "cases"+ext)
		data := []byte(fixture + "\n")
		if ext == ".json" {
			data = []byte(`{"cases":[` + fixture + `]}`)
		}
		if err = os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		got, err := predictCorpus(m, path, map[string]int{"bias": 0, "mora_progress": 1}, c)
		if err != nil {
			t.Fatal(err)
		}
		cases := got["cases"].([]map[string]any)
		if len(cases) != 1 || cases[0]["id"] != "sample" || cases[0]["reading"] != "アイ" {
			t.Fatalf("cases=%v", cases)
		}
		cents := cases[0]["cents"].([]float64)
		if len(cents) != 20 {
			t.Fatalf("frames=%d", len(cents))
		}
		for _, v := range cents {
			if math.IsNaN(v) || v < -250 || v > 250 {
				t.Fatalf("cents=%v", v)
			}
		}
	}
}
