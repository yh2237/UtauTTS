package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/yh2237/gograd/tensor"
	"utautts/internal/frontend"
	"utautts/internal/openjtalk"
	"utautts/internal/prosody"
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

func TestOpenJTalkReanalysis(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	helper := filepath.Join(root, "out", "gui-standard", "runtime", "utautts-openjtalk-features.exe")
	dict := filepath.Join(root, "out", "gui-standard", "runtime", "open_jtalk_dic_utf_8-1.11")
	if _, err := os.Stat(helper); err != nil {
		t.Skipf("Open JTalk helper unavailable: %v", err)
	}
	rows, err := loadRecords(filepath.Join(root, "out", "mfa-align-20261002", "base-mfa.jsonl"))
	if err != nil {
		t.Skipf("v10 dataset unavailable: %v", err)
	}
	before := rows[0]
	got, err := reanalyzeRecords(rows[:3], openjtalk.Config{HelperPath: helper, DictionaryPath: dict})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("all three records failed Open JTalk alignment")
	}
	for _, r := range got {
		var original record
		for _, source := range rows[:3] {
			if source.ID == r.ID {
				original = source
			}
		}
		if len(r.Tokens) != len(original.Tokens) {
			t.Fatal("token count changed")
		}
		for i, v := range r.Tokens {
			if v.Mora != original.Tokens[i].Mora || v.Pause != original.Tokens[i].Pause || v.Start != original.Tokens[i].Start || v.End != original.Tokens[i].End {
				t.Fatalf("timed alignment changed at %s/%d", r.ID, i)
			}
		}
		times := timeGrid(r, 10)
		maxDiff := 0.0
		for _, i := range []int{0, len(times) / 2, len(times) - 1} {
			originalFrame := frameFeatures(original, tokenAt(original.Tokens, times[i]), times[i], times[0]-5, times[len(times)-1]+5)
			newFrame := frameFeatures(r, tokenAt(r.Tokens, times[i]), times[i], times[0]-5, times[len(times)-1]+5)
			for name, v := range originalFrame {
				maxDiff = math.Max(maxDiff, math.Abs(newFrame[name]-v))
			}
		}
		t.Logf("%s Open JTalk feature max difference from aligned corpus: %.9g", r.ID, maxDiff)
	}
	if before.ID == "" {
		t.Fatal("missing fixture")
	}
	// The helper result must remain usable by the frame feature builder.
	f := frameFeatures(got[0], 0, got[0].Tokens[0].Start+5, got[0].Start, got[0].End)
	if f["bias"] != 1 {
		t.Fatalf("features=%v", f)
	}
}

func TestPredictCorpusRawTextAndFallback(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	helper := filepath.Join(root, "out", "gui-standard", "runtime", "utautts-openjtalk-features.exe")
	dict := filepath.Join(root, "out", "gui-standard", "runtime", "open_jtalk_dic_utf_8-1.11")
	m, err := newTCN(2, 3, []int{1}, tensor.CPU, 1)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "raw.jsonl")
	if err = os.WriteFile(path, []byte("{\"text\":\"こんにちは。\"}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c := config{Language: "ja", Frame: 10, Low: -250, High: 250, OpenJTalkHelper: helper, OpenJTalkDictionary: dict}
	result, err := predictCorpus(m, path, map[string]int{"bias": 0, "mora_progress": 1}, c)
	if err != nil {
		t.Fatal(err)
	}
	cases := result["cases"].([]map[string]any)
	if len(cases) != 1 || len(cases[0]["cents"].([]float64)) == 0 || cases[0]["reading"] == "こんにちは。" {
		t.Fatalf("raw text analysis=%v", cases)
	}
	c.OpenJTalkHelper = filepath.Join(t.TempDir(), "missing-helper.exe")
	result, err = predictCorpus(m, path, map[string]int{"bias": 0, "mora_progress": 1}, c)
	if err != nil {
		t.Fatal(err)
	}
	cases = result["cases"].([]map[string]any)
	if len(cases[0]["cents"].([]float64)) != 60 {
		t.Fatalf("fallback frames=%d", len(cases[0]["cents"].([]float64)))
	}
}

func TestCUDAExportLoadsInProsody(t *testing.T) {
	path := filepath.Join("..", "..", "..", "out", "frame-intonation-go", "v10-cuda-local-fix.json")
	if _, err := os.Stat(path); err != nil {
		t.Skip(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err = json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	model, err := prosody.ParseModel(data)
	if err != nil {
		t.Fatal(err)
	}
	curve := model.PredictFrameContour([]frontend.Mora{{Text: "ア", Vowel: "a"}}, nil, []prosody.MoraTiming{{StartMS: 0, DurationMS: 100}}, 100, false)
	if curve == nil || len(curve.Cents) == 0 {
		t.Fatal("no CUDA export contour")
	}
}
