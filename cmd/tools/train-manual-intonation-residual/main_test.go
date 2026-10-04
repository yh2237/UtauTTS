package main

import (
	"encoding/json"
	"math"
	"os"
	"testing"

	"utautts/internal/frontend"
	"utautts/internal/openjtalk"
	"utautts/internal/prosody"
)

func TestTimingFallback(t *testing.T) {
	start, duration := timing(map[string]any{}, []string{"あ", "", "い"}, []bool{false, true, false})
	if len(start) != 3 || start[0] != 0 || start[1] != 120 || start[2] != 300 || duration[1] != 180 {
		t.Fatalf("fallback timing: %v %v", start, duration)
	}
}

func TestPythonSampleParity(t *testing.T) {
	root := "../../../out/python-migration/"
	b, e := os.ReadFile(root + "manual-samples-py.json")
	if e != nil {
		t.Skip("optional Python Lab fixture absent")
	}
	var expected struct {
		Accepted int      `json:"accepted"`
		Phrases  int      `json:"phrases"`
		First    phrase   `json:"first"`
		Rows     []phrase `json:"rows"`
	}
	if e = json.Unmarshal(b, &expected); e != nil {
		t.Fatal(e)
	}
	cfg := openjtalk.Config{HelperPath: "../../../tools/openjtalk-feature-bridge/bin/utautts-openjtalk-features.exe", DictionaryPath: "../../../.tmp-openjtalk/pyopenjtalk/open_jtalk_dic_utf_8-1.11"}
	got, accepted, skipped, e := loadPhrases([]string{root + "manual-fixture.utautts"}, 180, cfg)
	if e != nil {
		t.Fatal(e)
	}
	if accepted != expected.Accepted || len(got) != expected.Phrases || len(skipped) != 0 {
		t.Fatalf("accepted=%d phrases=%d skipped=%v", accepted, len(got), skipped)
	}
	for p, wantPhrase := range expected.Rows {
		if got[p].ID != wantPhrase.ID || len(got[p].X) != len(wantPhrase.X) || len(got[p].Y) != len(wantPhrase.Y) {
			t.Fatalf("phrase %d structure differs", p)
		}
		for i, want := range wantPhrase.X {
			if len(got[p].X[i]) != len(want) || math.Abs(got[p].Y[i]-wantPhrase.Y[i]) > 1e-8 {
				missing := []string{}
				for name := range want {
					if _, ok := got[p].X[i][name]; !ok {
						missing = append(missing, name)
					}
				}
				t.Fatalf("phrase %d row %d differs: features=%d/%d target=%.12g/%.12g missing=%v", p, i, len(got[p].X[i]), len(want), got[p].Y[i], wantPhrase.Y[i], missing)
			}
			for name, value := range want {
				if math.Abs(got[p].X[i][name]-value) > 1e-8 {
					t.Fatalf("phrase %d row %d feature %s differs", p, i, name)
				}
			}
		}
	}
	first := got[0]
	if first.ID != expected.First.ID || len(first.X) != len(expected.First.X) {
		t.Fatal("first phrase differs")
	}
	for i, want := range expected.First.X {
		for name, value := range want {
			if math.Abs(first.X[i][name]-value) > 1e-8 {
				t.Fatalf("feature %d %s: %.12g vs %.12g", i, name, first.X[i][name], value)
			}
		}
		if len(first.X[i]) != len(want) || math.Abs(first.Y[i]-expected.First.Y[i]) > 1e-8 {
			t.Fatalf("row %d differs", i)
		}
	}
}

func TestExportLoadsAndPredicts(t *testing.T) {
	path := "../../../out/python-migration/manual-go20.json"
	if _, e := os.Stat(path); e != nil {
		t.Skip("optional trained Lab fixture absent")
	}
	m, e := prosody.LoadModel(path)
	if e != nil {
		t.Fatal(e)
	}
	morae, e := frontend.ParseKana("みずを")
	if e != nil {
		t.Fatal(e)
	}
	timings := make([]prosody.MoraTiming, len(morae))
	for i := range timings {
		timings[i] = prosody.MoraTiming{StartMS: float64(i * 120), DurationMS: 120}
	}
	contour := m.PredictFrameContour(morae, nil, timings, float64(len(morae)*120), false)
	if contour == nil || len(contour.Cents) == 0 {
		t.Fatal("empty contour")
	}
}
