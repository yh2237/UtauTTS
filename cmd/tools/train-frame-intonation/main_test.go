package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/yh2237/gograd/tensor"
	"utautts/internal/frontend"
	"utautts/internal/prosody"
)

func TestPythonFeatureAndTargetParity(t *testing.T) {
	path := filepath.Join("..", "..", "..", "out", "mfa-align-20261002", "base-mfa.jsonl")
	rows, e := loadRecords(path)
	if e != nil {
		t.Skipf("v10 dataset unavailable: %v", e)
	}
	cache := filepath.Join("..", "..", "..", "out", "mfa-align-20261002", "f0-internal")
	wantFrames := []int{679, 730, 473}
	wantVoiced := []int{511, 551, 427}
	wantTargets := [][]float64{{-1, -1, -1, 0, 0, 1, 0}, {-1, -1, -1, .08693734, .22061683, -.18419842, .27407464}, {-.51204836, -.45906273, .036695, 1, 1, -1, -1}}
	positions := []int{0, 1, 5, 20, 50, 100, 200}
	for k := 0; k < 3; k++ {
		r := rows[k]
		if k == 0 {
			got := filepath.Base(cachePath(cache, r, 10))
			if got != "9938fe180d1b278270e82c2acaf86a7c368ea2b6.npy" {
				t.Fatalf("cache key %s", got)
			}
			times := timeGrid(r, 10)
			f := frameFeatures(r, tokenAt(r.Tokens, times[0]), times[0], times[0]-5, times[len(times)-1]+5)
			for name, want := range map[string]float64{"accent_position": .5, "accent_from_end": .5, "frame_position": .000736377, "mora_progress": .031250001, "mora_progress2": .000976563} {
				if math.Abs(f[name]-want) > 1e-6 {
					t.Errorf("%s: %.9f want %.9f", name, f[name], want)
				}
			}
		}
		times := timeGrid(r, 10)
		f0, e := npyF64(cachePath(cache, r, 10))
		if e != nil {
			t.Fatal(e)
		}
		if len(f0) != len(times) {
			t.Fatalf("%s cache frames %d want %d", r.ID, len(f0), len(times))
		}
		mask := make([]bool, len(times))
		for i, x := range times {
			mask[i] = !r.Tokens[tokenAt(r.Tokens, x)].Pause
		}
		target := targetF0(f0, mask, 10, 40, -250, 250)
		if len(target) != wantFrames[k] {
			t.Errorf("%s frames %d", r.ID, len(target))
		}
		n := 0
		for i := range mask {
			if mask[i] && target[i] != 0 {
				n++
			}
		}
		_ = n
		for j, i := range positions {
			if math.Abs(float64(target[i])-wantTargets[k][j]) > 3e-4 {
				t.Errorf("%s target[%d]=%.7f want %.7f", r.ID, i, target[i], wantTargets[k][j])
			}
		}
		_ = wantVoiced
	}
}

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

func TestPythonFixtureFullParity(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	path := filepath.Join(root, "out", "frame-intonation-go", "python-parity.json")
	data, e := os.ReadFile(path)
	if e != nil {
		t.Skipf("optional Python fixture: %v", e)
	}
	var reference []struct {
		ID     string                        `json:"id"`
		Target []float64                     `json:"target"`
		Mask   []bool                        `json:"mask"`
		Frames map[string]map[string]float64 `json:"frames"`
	}
	if e = json.Unmarshal(data, &reference); e != nil {
		t.Fatal(e)
	}
	rows, e := loadRecords(filepath.Join(root, "out", "mfa-align-20261002", "base-mfa.jsonl"))
	if e != nil {
		t.Fatal(e)
	}
	index := map[string]int{}
	for i, name := range featureNames(rows[:3], 10) {
		index[name] = i
	}
	maxTarget, maxFeature := 0.0, 0.0
	for k, ref := range reference {
		r := rows[k]
		if r.ID != ref.ID {
			t.Fatal("fixture ID mismatch")
		}
		ex, e := prepareRecord(r, index, 10, 40, -250, 250, filepath.Join(root, "out", "mfa-align-20261002", "f0-internal"))
		if e != nil {
			t.Fatal(e)
		}
		if len(ex.Targets) != len(ref.Target) {
			t.Fatal("frame mismatch")
		}
		for i, v := range ref.Target {
			d := math.Abs(float64(ex.Targets[i]) - v)
			maxTarget = math.Max(maxTarget, d)
			if d > 1e-5 {
				t.Errorf("%s target frame %d: %g", r.ID, i, d)
				break
			}
			if ex.Mask[i] != ref.Mask[i] {
				t.Errorf("%s mask frame %d", r.ID, i)
				break
			}
		}
		times := timeGrid(r, 10)
		for key, features := range ref.Frames {
			i, e := strconv.Atoi(key)
			if e != nil {
				t.Fatal(e)
			}
			got := frameFeatures(r, tokenAt(r.Tokens, times[i]), times[i], times[0]-5, times[len(times)-1]+5)
			if len(got) != len(features) {
				t.Errorf("%s frame %d feature count %d want %d", r.ID, i, len(got), len(features))
			}
			for name, v := range features {
				d := math.Abs(got[name] - v)
				maxFeature = math.Max(maxFeature, d)
				if d > 1e-6 {
					t.Errorf("%s frame %d %s: %g", r.ID, i, name, d)
				}
			}
		}
	}
	t.Logf("3 utterances: max target difference %.9g normalized cents, max feature difference %.9g", maxTarget, maxFeature)
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

func TestEnglishPythonFeatureParity(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	data, e := os.ReadFile(filepath.Join(root, "out", "frame-intonation-go", "python-english-features.json"))
	if e != nil {
		t.Skip(e)
	}
	var refs []struct {
		ID       string                        `json:"id"`
		Features map[string]map[string]float64 `json:"features"`
	}
	if e = json.Unmarshal(data, &refs); e != nil {
		t.Fatal(e)
	}
	rows, e := loadRecords(filepath.Join(root, "out", "english-frame-v1", "corpus.jsonl"))
	if e != nil {
		t.Fatal(e)
	}
	maxDiff := 0.0
	for k, ref := range refs {
		r := rows[k]
		if r.ID != ref.ID {
			t.Fatal("ID mismatch")
		}
		times := timeGrid(r, 10)
		for key, want := range ref.Features {
			i, e := strconv.Atoi(key)
			if e != nil {
				t.Fatal(e)
			}
			got := frameFeatures(r, tokenAt(r.Tokens, times[i]), times[i], times[0]-5, times[len(times)-1]+5)
			if len(got) != len(want) {
				t.Errorf("%s frame %d feature count %d != %d", r.ID, i, len(got), len(want))
			}
			for name, value := range want {
				diff := math.Abs(got[name] - value)
				maxDiff = math.Max(maxDiff, diff)
				if diff > 1e-6 {
					t.Errorf("%s frame %d %s diff %.9g", r.ID, i, name, diff)
				}
			}
		}
	}
	t.Logf("3 English utterances: maximum feature difference %.9g", maxDiff)
}

func TestInternalF0PythonParity(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	rows, e := loadRecords(filepath.Join(root, "out", "mfa-align-20261002", "base-mfa.jsonl"))
	if e != nil {
		t.Skip(e)
	}
	r := rows[0]
	track, e := internalF0(r, 10)
	if e != nil {
		t.Fatal(e)
	}
	times := timeGrid(r, 10)
	got := interpolateF0(track, times, 10)
	for i, x := range times {
		if r.Tokens[tokenAt(r.Tokens, x)].Pause {
			got[i] = 0
		}
	}
	want, e := npyF64(cachePath(filepath.Join(root, "out", "mfa-align-20261002", "f0-internal"), r, 10))
	if e != nil {
		t.Fatal(e)
	}
	if len(got) != len(want) {
		t.Fatalf("frames %d != %d", len(got), len(want))
	}
	maxDiff := 0.0
	different := 0
	for i := range got {
		d := math.Abs(got[i] - want[i])
		maxDiff = math.Max(maxDiff, d)
		if d > 1e-3 {
			different++
		}
	}
	t.Logf("internal F0: %d frames, %d differ >0.001 Hz, max difference %.6f Hz", len(got), different, maxDiff)
	if different > 0 {
		t.Fail()
	}
}
