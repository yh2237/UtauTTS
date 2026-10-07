package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/gob"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"utautts/internal/speechtiming"

	"github.com/yh2237/gograd/autograd"
	"github.com/yh2237/gograd/cuda"
)

func featureFixture(t *testing.T) string {
	t.Helper()
	var items []utterance
	for i := 0; i < 4; i++ {
		frames := 3 + 2*i
		item := utterance{ID: string(rune('a' + i)), Frames: frames, Continuous: 4,
			IDs: make([]int, frames*3), Cont: make([]float32, frames*4), Target: make([]float32, frames*80),
			F0Target: make([]float32, frames), EnergyTarget: make([]float32, frames), F0Extra: make([]float32, frames*f0ExtraFeatures)}
		for f := 0; f < frames; f++ {
			for j := 0; j < 3; j++ {
				item.IDs[f*3+j] = 3 + (i+f+j)%20
			}
			for j := 0; j < 4; j++ {
				item.Cont[f*4+j] = float32((f+j)%7) / 7
			}
			for j := 0; j < 80; j++ {
				item.Target[f*80+j] = float32(math.Sin(float64(i+f+j) * .1))
			}
			item.F0Target[f] = float32(math.Sin(float64(i+f)*.2)) * 0.5
			item.EnergyTarget[f] = float32(math.Cos(float64(i+f)*.15)) * 0.3
			for j := 0; j < f0ExtraFeatures; j++ {
				item.F0Extra[f*f0ExtraFeatures+j] = float32((i+f+j)%5) / 5
			}
		}
		items = append(items, item)
	}
	var data bytes.Buffer
	if err := gob.NewEncoder(&data).Encode(featureCache{Version: featureVersion, Phones: phoneNames, Data: items}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "features.gob")
	if err := os.WriteFile(path, data.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTrainerResumeAndRuntimeCompatibility(t *testing.T) {
	for _, device := range []string{"cpu", "cuda"} {
		t.Run(device, func(t *testing.T) {
			if device == "cuda" && !cuda.Available() {
				t.Skip("CUDA unavailable")
			}
			cache := featureFixture(t)
			c := trainingConfig{Cache: cache, Steps: 6, Valid: 1, Batch: 1, Window: 5, EvalEvery: 2, CheckpointEvery: 99, LR: 0.002,
				Seed: 17, Device: device, TrainingCorpus: "test corpus", Notices: noticeFlags{"test-notice.txt"}}
			c.Out = filepath.Join(t.TempDir(), "full.safetensors")
			c.Fixture = c.Out + ".fixture.json"
			if err := train(context.Background(), c); err != nil {
				t.Fatal(err)
			}
			full, fullFixture := c.Out, c.Fixture
			c.Out = filepath.Join(t.TempDir(), "partial.safetensors")
			c.Fixture = c.Out + ".fixture.json"
			c.StopAfter = 4
			if err := train(context.Background(), c); err != nil {
				t.Fatal(err)
			}
			partialMetadata, err := autograd.TrainingCheckpointMetadata(c.Out + ".training.safetensors")
			if err != nil {
				t.Fatal(err)
			}
			var partialState trainingState
			if err := json.Unmarshal([]byte(partialMetadata["trainer"]), &partialState); err != nil {
				t.Fatal(err)
			}
			if partialState.BestStep >= c.StopAfter-1 {
				t.Fatal("fixture does not cover distinct current and best weights")
			}
			c.Resume = c.Out + ".training.safetensors"
			c.Out = filepath.Join(t.TempDir(), "resumed.safetensors")
			c.Fixture = c.Out + ".fixture.json"
			c.StopAfter = 0
			if err := train(context.Background(), c); err != nil {
				t.Fatal(err)
			}
			for _, suffix := range []string{"", ".training.safetensors"} {
				want, err := os.ReadFile(full + suffix)
				if err != nil {
					t.Fatal(err)
				}
				got, err := os.ReadFile(c.Out + suffix)
				if err != nil {
					t.Fatal(err)
				}
				if device == "cpu" {
					if !bytes.Equal(want, got) {
						t.Fatalf("resumed checkpoint differs byte-for-byte: %s", suffix)
					}
				} else {
					compareCUDACheckpoint(t, want, got)
				}
			}
			if device == "cpu" {
				want, _ := os.ReadFile(fullFixture)
				got, _ := os.ReadFile(c.Fixture)
				if !bytes.Equal(want, got) {
					t.Fatal("resumed parity fixture differs")
				}
			}
			verifyRuntime(t, c.Out, c.Fixture)
			metadata, err := autograd.TrainingCheckpointMetadata(c.Out + ".training.safetensors")
			if err != nil {
				t.Fatal(err)
			}
			if metadata["tool"] != trainingFormat {
				t.Fatal("missing trainer identity")
			}
			// 全体の予定step数を変えるとOneCycleが変わるため拒否する。
			c.Out = filepath.Join(t.TempDir(), "invalid.safetensors")
			c.Fixture = c.Out + ".fixture.json"
			c.Steps++
			if err := train(context.Background(), c); err == nil {
				t.Fatal("accepted changed schedule")
			}
			if _, err := os.Stat(c.Out); !os.IsNotExist(err) {
				t.Fatal("invalid resume created output")
			}
			c.Steps--
			c.TrainingCorpus = "changed provenance"
			if err := train(context.Background(), c); err == nil {
				t.Fatal("accepted changed provenance")
			}
			c.TrainingCorpus = "test corpus"
			file, err := os.OpenFile(cache, os.O_APPEND|os.O_WRONLY, 0600)
			if err != nil {
				t.Fatal(err)
			}
			file.Write([]byte{0})
			file.Close()
			if err := train(context.Background(), c); err == nil {
				t.Fatal("accepted changed cache")
			}
		})
	}
}

func verifyRuntime(t *testing.T, modelPath, fixturePath string) {
	t.Helper()
	data, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	model, err := speechtiming.LoadTCN(data)
	if err != nil {
		t.Fatalf("new checkpoint is incompatible with runtime: %v", err)
	}
	if model.Mels() != 80 || len(model.Phones()) != 40 {
		t.Fatal("model metadata changed")
	}
	data, err = os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	var fixture parityFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	values, err := model.Predict(fixture.IDs, fixture.Cont)
	if err != nil {
		t.Fatal(err)
	}
	maximum := 0.0
	for i := range values {
		for j := range values[i] {
			if math.IsNaN(float64(values[i][j])) || math.IsInf(float64(values[i][j]), 0) {
				t.Fatal("non-finite runtime prediction")
			}
			maximum = math.Max(maximum, math.Abs(float64(values[i][j]-fixture.Output[i][j])))
		}
	}
	t.Logf("runtime/parity max absolute error = %g", maximum)
	if maximum > 2e-5 {
		t.Fatalf("runtime predictions differ from best model: %g", maximum)
	}
	if len(fixture.F0Output) > 0 {
		if !model.HasF0Head() || model.F0Context() != len(fixture.F0Cont[0]) {
			t.Fatal("runtime model is missing the F0 head")
		}
		cont := make([][]float32, len(fixture.F0Cont))
		for i, row := range fixture.F0Cont {
			cont[i] = row[:]
		}
		f0Values, err := model.PredictF0(fixture.IDs, cont)
		if err != nil {
			t.Fatal(err)
		}
		if len(f0Values) != len(fixture.F0Output) {
			t.Fatalf("runtime f0 frames %d != %d", len(f0Values), len(fixture.F0Output))
		}
		f0Maximum := 0.0
		for i := range f0Values {
			if math.IsNaN(float64(f0Values[i])) || math.IsInf(float64(f0Values[i]), 0) {
				t.Fatal("non-finite runtime f0 prediction")
			}
			f0Maximum = math.Max(f0Maximum, math.Abs(float64(f0Values[i]-fixture.F0Output[i])))
		}
		t.Logf("runtime/parity f0 max absolute error = %g", f0Maximum)
		if f0Maximum > 2e-5 {
			t.Fatalf("runtime f0 predictions differ from best model: %g", f0Maximum)
		}
		if len(fixture.EnergyOutput) > 0 {
			energyValues, err := model.PredictEnergy(fixture.IDs, cont)
			if err != nil {
				t.Fatal(err)
			}
			if len(energyValues) != len(fixture.EnergyOutput) {
				t.Fatalf("runtime energy frames %d != %d", len(energyValues), len(fixture.EnergyOutput))
			}
			energyMaximum := 0.0
			for i := range energyValues {
				if math.IsNaN(float64(energyValues[i])) || math.IsInf(float64(energyValues[i]), 0) {
					t.Fatal("non-finite runtime energy prediction")
				}
				energyMaximum = math.Max(energyMaximum, math.Abs(float64(energyValues[i]-fixture.EnergyOutput[i])))
			}
			t.Logf("runtime/parity energy max absolute error = %g", energyMaximum)
			if energyMaximum > 2e-5 {
				t.Fatalf("runtime energy predictions differ from best model: %g", energyMaximum)
			}
		}
	}
}

func TestFeatureOnlyDoesNotNeedMFAWithCache(t *testing.T) {
	c := trainingConfig{Cache: featureFixture(t), FeaturesOnly: true, FeaturesJSON: filepath.Join(t.TempDir(), "features.json")}
	if err := train(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(c.FeaturesJSON)
	if err != nil {
		t.Fatal(err)
	}
	var items []utterance
	if err := json.Unmarshal(data, &items); err != nil || len(items) != 3 {
		t.Fatal("feature export changed")
	}
}

func TestMultiHeadTrainingWritesF0Model(t *testing.T) {
	cache := featureFixture(t)
	c := trainingConfig{Cache: cache, F0Head: true, F0Weight: 1, EnergyWeight: 1, LR: 0.002, Steps: 6, Valid: 1, Batch: 1, Window: 5, EvalEvery: 2, CheckpointEvery: 99,
		Seed: 17, Device: "cpu", TrainingCorpus: "test corpus", Notices: noticeFlags{"test-notice.txt"}}
	c.Out = filepath.Join(t.TempDir(), "multi.safetensors")
	c.Fixture = c.Out + ".fixture.json"
	if err := train(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(c.Out)
	if err != nil {
		t.Fatal(err)
	}
	headerLength := binary.LittleEndian.Uint64(data[:8])
	var header map[string]json.RawMessage
	if err := json.Unmarshal(data[8:8+headerLength], &header); err != nil {
		t.Fatal(err)
	}
	var metadata map[string]string
	if err := json.Unmarshal(header["__metadata__"], &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata["f0"] != "1" || metadata["f0_context"] != "55" || metadata["energy"] != "1" {
		t.Fatalf("f0 metadata = %v", metadata)
	}
	for _, name := range []string{"f0_inp.weight", "f0_blocks.0.weight", "f0_out.weight", "energy_out.weight"} {
		if _, ok := header[name]; !ok {
			t.Errorf("missing %s", name)
		}
	}
	fixtureData, err := os.ReadFile(c.Fixture)
	if err != nil {
		t.Fatal(err)
	}
	var fixture parityFixture
	if err := json.Unmarshal(fixtureData, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.F0Output) != 37 || len(fixture.F0Cont) != 37 || len(fixture.EnergyOutput) != 37 {
		t.Fatalf("fixture f0 output %d f0 cont %d energy %d", len(fixture.F0Output), len(fixture.F0Cont), len(fixture.EnergyOutput))
	}
	verifyRuntime(t, c.Out, c.Fixture)
}

func TestTrainingProtectsExistingPaths(t *testing.T) {
	cache := featureFixture(t)
	c := trainingConfig{Cache: cache, Out: cache, Fixture: filepath.Join(t.TempDir(), "fixture.json"), Steps: 6, Valid: 1, Batch: 1, Window: 5, EvalEvery: 1, CheckpointEvery: 1, Device: "cpu", LR: 0.002}
	before, _ := os.ReadFile(cache)
	if err := train(context.Background(), c); err == nil {
		t.Fatal("accepted output over feature cache")
	}
	after, _ := os.ReadFile(cache)
	if !bytes.Equal(before, after) {
		t.Fatal("feature cache was modified")
	}
	c.Out = filepath.Join(t.TempDir(), "model.safetensors")
	c.Fixture = c.Out
	if err := train(context.Background(), c); err == nil {
		t.Fatal("accepted colliding output paths")
	}
}

// CUDAの勾配集約順によるfloat32の差だけを許す。乱数・分割・schedulerは完全一致。
func compareCUDACheckpoint(t *testing.T, want, got []byte) {
	t.Helper()
	decode := func(file []byte) (map[string]safeTestEntry, map[string]string, []byte) {
		n := int(binary.LittleEndian.Uint64(file[:8]))
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(file[8:8+n], &raw); err != nil {
			t.Fatal(err)
		}
		var metadata map[string]string
		if err := json.Unmarshal(raw["__metadata__"], &metadata); err != nil {
			t.Fatal(err)
		}
		delete(raw, "__metadata__")
		entries := map[string]safeTestEntry{}
		for name, rawEntry := range raw {
			var entry safeTestEntry
			if err := json.Unmarshal(rawEntry, &entry); err != nil {
				t.Fatal(err)
			}
			entries[name] = entry
		}
		return entries, metadata, file[8+n:]
	}
	a, am, ad := decode(want)
	b, bm, bd := decode(got)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("CUDA checkpoint tensor layout changed")
	}
	if am["format"] == "gograd-training-1" {
		var au, bu map[string]string
		json.Unmarshal([]byte(am["user"]), &au)
		json.Unmarshal([]byte(bm["user"]), &bu)
		var as, bs trainingState
		json.Unmarshal([]byte(au["trainer"]), &as)
		json.Unmarshal([]byte(bu["trainer"]), &bs)
		if math.Abs(as.Best-bs.Best) > 1e-6 {
			t.Fatal("CUDA validation diverged")
		}
		bs.Best = as.Best
		if !reflect.DeepEqual(as, bs) {
			t.Fatal("CUDA trainer/sampler state diverged")
		}
		delete(am, "user")
		delete(bm, "user")
	}
	if !reflect.DeepEqual(am, bm) {
		t.Fatal("CUDA checkpoint metadata diverged")
	}
	maximum := 0.0
	for _, entry := range a {
		for i := entry.Offsets[0]; i < entry.Offsets[1]; i += 4 {
			x := math.Float32frombits(binary.LittleEndian.Uint32(ad[i:]))
			y := math.Float32frombits(binary.LittleEndian.Uint32(bd[i:]))
			if math.IsNaN(float64(x)) || math.IsNaN(float64(y)) || math.IsInf(float64(x), 0) || math.IsInf(float64(y), 0) {
				t.Fatal("non-finite checkpoint tensor")
			}
			maximum = math.Max(maximum, math.Abs(float64(x-y)))
		}
	}
	t.Logf("CUDA resumed checkpoint max absolute error = %g", maximum)
	if maximum > 1e-6 {
		t.Fatalf("CUDA resume error = %g", maximum)
	}
}

type safeTestEntry struct {
	DType   string `json:"dtype"`
	Shape   []int  `json:"shape"`
	Offsets [2]int `json:"data_offsets"`
}
