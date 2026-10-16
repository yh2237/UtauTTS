package main

import (
	encodingbinary "encoding/binary"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/yh2237/gograd/autograd"
	"github.com/yh2237/gograd/cuda"
	"github.com/yh2237/gograd/tensor"
)

func TestSpeechTimingPyTorchParity(t *testing.T) {
	requirePyTorch(t)
	dir := t.TempDir()
	cmd := exec.Command("python", "testdata/gen_model_fixture.py", "--output", dir)
	if reference := os.Getenv("GOGRAD_SPEECH_REFERENCE"); reference != "" {
		cmd.Args = append(cmd.Args, "--reference", reference)
	}
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	if out, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("fixture: %v\n%s", e, out)
	}
	data, e := os.ReadFile(filepath.Join(dir, "fixture.json"))
	if e != nil {
		t.Fatal(e)
	}
	var f struct {
		IDs      []int                `json:"ids"`
		Cont     []float32            `json:"cont"`
		Target   []float32            `json:"target"`
		NanRows  []int                `json:"nan_rows"`
		Output   []float32            `json:"output"`
		Loss     float32              `json:"loss"`
		ClipNorm float32              `json:"clip_norm"`
		Grads    map[string][]float32 `json:"grads"`
		Post     map[string][]float32 `json:"post"`
	}
	if e = json.Unmarshal(data, &f); e != nil {
		t.Fatal(e)
	}
	for _, row := range f.NanRows {
		for j := 0; j < 80; j++ {
			f.Target[row*80+j] = float32(math.NaN())
		}
	}
	for _, device := range []tensor.Device{tensor.CPU, tensor.CUDA} {
		t.Run(string(device), func(t *testing.T) {
			if device == tensor.CUDA && !cuda.Available() {
				t.Skip("CUDA unavailable")
			}
			if device == tensor.CUDA {
				ctx, e := autograd.NewCUDAContext()
				if e != nil {
					t.Fatal(e)
				}
				defer ctx.Close()
			}
			m, e := NewSpeechTiming(4, device, 0)
			if e != nil {
				t.Fatal(e)
			}
			if e = m.Module.LoadSafeTensors(filepath.Join(dir, "weights.safetensors")); e != nil {
				t.Fatal(e)
			}
			cont, e := autograd.New(f.Cont, []int{2, 7, 4}, device, false)
			if e != nil {
				t.Fatal(e)
			}
			target, e := autograd.New(f.Target, []int{2, 7, 80}, device, false)
			if e != nil {
				t.Fatal(e)
			}
			pred := m.Forward(f.IDs, cont, 99)
			if device == tensor.CUDA && (pred.Buffer() == nil || pred.Data != nil) {
				t.Fatal("speech timing forward left CUDA device")
			}
			got, e := pred.ToHost()
			if e != nil {
				t.Fatal(e)
			}
			checkSpeechValues(t, "output", got, f.Output, 1e-5)
			outputErr := speechMaxDiff(got, f.Output)
			loss := autograd.MaskedLoss(pred, target, false)
			got, e = loss.ToHost()
			if e != nil {
				t.Fatal(e)
			}
			checkSpeechValues(t, "loss", got, []float32{f.Loss}, 1e-5)
			lossErr := speechMaxDiff(got, []float32{f.Loss})
			if e = loss.Backward(); e != nil {
				t.Fatal(e)
			}
			var gradErr float64
			for _, p := range m.Parameters() {
				g, e := p.Value.GradToHost()
				if e != nil {
					t.Fatal(e)
				}
				checkSpeechValues(t, p.Name+" gradient", g, f.Grads[p.Name], 1e-5)
				gradErr = math.Max(gradErr, speechMaxDiff(g, f.Grads[p.Name]))
			}
			params := m.Parameters()
			norm := autograd.ClipGradNorm(params, 1)
			if device == tensor.CUDA {
				norm = autograd.GradientNormToHost()
			}
			checkSpeechValues(t, "clip norm", []float32{norm}, []float32{f.ClipNorm}, 1e-4)
			opt := autograd.NewAdamW(params, .002, .0001)
			opt.Step()
			var stepErr float64
			for _, p := range params {
				v, e := p.Value.ToHost()
				if e != nil {
					t.Fatal(e)
				}
				checkSpeechValues(t, p.Name+" AdamW", v, f.Post[p.Name], 2e-4)
				stepErr = math.Max(stepErr, speechMaxDiff(v, f.Post[p.Name]))
			}
			t.Logf("max abs: forward %.6g loss %.6g gradients %.6g AdamW %.6g", outputErr, lossErr, gradErr, stepErr)
			loss.ReleaseGraph()
			cont.Close()
			target.Close()
		})
	}
}

func checkSpeechValues(t *testing.T, label string, got, want []float32, tol float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s length %d != %d", label, len(got), len(want))
	}
	var maxAbs float64
	at := 0
	for i := range got {
		d := math.Abs(float64(got[i] - want[i]))
		if d > maxAbs {
			maxAbs, at = d, i
		}
	}
	if maxAbs > tol {
		t.Errorf("%s max abs %.6g at %d: got %.6g want %.6g", label, maxAbs, at, got[at], want[at])
	}
}

func speechMaxDiff(got, want []float32) float64 {
	var maxAbs float64
	for i := range got {
		maxAbs = math.Max(maxAbs, math.Abs(float64(got[i]-want[i])))
	}
	return maxAbs
}

func TestSpeechTimingOneCyclePyTorch(t *testing.T) {
	// torch.optim.lr_scheduler.OneCycleLR(max_lr=.002,total_steps=6000,pct_start=.1)
	want := map[int]float64{0: .00008, 1: .00008001320341708383, 599: .002,
		600: .0019999998307687816, 3000: .0011730785180852907, 5999: .000000008}
	c := autograd.NewOneCycle(.002, 6000, .1)
	for i := 0; i < 6000; i++ {
		if expected, ok := want[i]; ok && math.Abs(c.LR()-expected) > 1e-10 {
			t.Errorf("step %d LR %.12g != PyTorch %.12g", i, c.LR(), expected)
		}
		c.StepCount++
	}
}

func TestSpeechTimingMultiHeadShapesAndParameters(t *testing.T) {
	m, e := NewSpeechTimingMultiHead(44, 4, 2, tensor.CPU, 0)
	if e != nil {
		t.Fatal(e)
	}
	cont, e := autograd.New(make([]float32, 2*5*4), []int{2, 5, 4}, tensor.CPU, false)
	if e != nil {
		t.Fatal(e)
	}
	f0Cont, e := autograd.New(make([]float32, 2*5*2), []int{2, 5, 2}, tensor.CPU, false)
	if e != nil {
		t.Fatal(e)
	}
	mel, f0, energy := m.Forward(make([]int, 2*5*3), cont, f0Cont, 7)
	if len(mel.Shape) != 3 || mel.Shape[2] != 80 || mel.Shape[0] != 2 || mel.Shape[1] != 5 {
		t.Fatalf("mel shape %v", mel.Shape)
	}
	if len(f0.Shape) != 3 || f0.Shape[2] != 1 || f0.Shape[0] != 2 || f0.Shape[1] != 5 {
		t.Fatalf("f0 shape %v", f0.Shape)
	}
	if len(energy.Shape) != 3 || energy.Shape[2] != 1 || energy.Shape[0] != 2 || energy.Shape[1] != 5 {
		t.Fatalf("energy shape %v", energy.Shape)
	}
	names := map[string]bool{}
	for _, p := range m.Parameters() {
		names[p.Name] = true
	}
	for _, want := range []string{"phone.weight", "inp.weight", "blocks.0.weight", "norms.0.weight", "out.weight", "f0_inp.weight", "f0_blocks.0.weight", "f0_norms.0.weight", "f0_out.weight", "energy_out.weight"} {
		if !names[want] {
			t.Errorf("missing parameter %q", want)
		}
	}
	if _, e := NewSpeechTimingMultiHead(44, 4, 0, tensor.CPU, 0); e == nil {
		t.Fatal("zero f0 context width was accepted")
	}
}

func TestSpeechTimingWithPhonesUsesVocabularySize(t *testing.T) {
	m, e := NewSpeechTimingWithPhones(44, 4, tensor.CPU, 0)
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "model.safetensors")
	if e = m.Module.SaveSafeTensorsMetadata(path, map[string]string{"format": "test"}); e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	hlen := encodingbinary.LittleEndian.Uint64(data[:8])
	var header map[string]json.RawMessage
	if e = json.Unmarshal(data[8:8+hlen], &header); e != nil {
		t.Fatal(e)
	}
	var entry struct {
		Shape []int `json:"shape"`
	}
	if e = json.Unmarshal(header["phone.weight"], &entry); e != nil {
		t.Fatal(e)
	}
	if len(entry.Shape) != 2 || entry.Shape[0] != 44 || entry.Shape[1] != 32 {
		t.Fatalf("phone embedding shape = %v, want [44 32]", entry.Shape)
	}
	if _, e := NewSpeechTimingWithPhones(0, 4, tensor.CPU, 0); e == nil {
		t.Fatal("zero phone count was accepted")
	}
}

func TestSpeechTimingCheckpointMetadata(t *testing.T) {
	m, e := NewSpeechTiming(4, tensor.CPU, 0)
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "model.safetensors")
	meta := map[string]string{"format": "utautts-speech-timing-tcn-1", "phones": "<pad> <unk>"}
	if e = m.Module.SaveSafeTensorsMetadata(path, meta); e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	hlen := encodingbinary.LittleEndian.Uint64(data[:8])
	var header map[string]json.RawMessage
	if e = json.Unmarshal(data[8:8+hlen], &header); e != nil {
		t.Fatal(e)
	}
	if len(header) != 38 {
		t.Fatalf("checkpoint entries %d, want 37 tensors plus metadata", len(header))
	}
	var got map[string]string
	if e = json.Unmarshal(header["__metadata__"], &got); e != nil {
		t.Fatal(e)
	}
	if got["format"] != meta["format"] {
		t.Fatalf("format %q", got["format"])
	}
	n, e := NewSpeechTiming(4, tensor.CPU, 1)
	if e != nil {
		t.Fatal(e)
	}
	if e = n.Module.LoadSafeTensors(path); e != nil {
		t.Fatal(e)
	}
	loaded := n.Module.StateDict()
	for name, values := range m.Module.StateDict() {
		checkSpeechValues(t, name, loaded[name], values, 0)
	}
}

func TestSpeechTimingMultiHeadCustomF0Dilations(t *testing.T) {
	dilations := []int{1, 2, 4, 8, 16, 32, 64}
	m, e := NewSpeechTimingMultiHeadWithF0Dilations(44, 4, 2, dilations, tensor.CPU, 0)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.F0Blocks) != len(dilations) || len(m.Blocks) != len(SpeechTimingDilations) {
		t.Fatalf("blocks: f0 %d mel %d", len(m.F0Blocks), len(m.Blocks))
	}
	names := map[string]bool{}
	for _, p := range m.Parameters() {
		names[p.Name] = true
	}
	if !names["f0_blocks.6.weight"] || names["f0_blocks.7.weight"] {
		t.Fatal("f0 block parameters do not follow the dilation schedule")
	}
	cont, e := autograd.New(make([]float32, 1*300*4), []int{1, 300, 4}, tensor.CPU, false)
	if e != nil {
		t.Fatal(e)
	}
	f0Cont, e := autograd.New(make([]float32, 1*300*2), []int{1, 300, 2}, tensor.CPU, false)
	if e != nil {
		t.Fatal(e)
	}
	if _, f0, _ := m.Forward(make([]int, 300*3), cont, f0Cont, 1); f0.Shape[1] != 300 {
		t.Fatalf("f0 shape %v", f0.Shape)
	}
	for _, bad := range [][]int{nil, {1, 0}} {
		if _, e := NewSpeechTimingMultiHeadWithF0Dilations(44, 4, 2, bad, tensor.CPU, 0); e == nil {
			t.Fatalf("dilations %v were accepted", bad)
		}
	}
}

var (
	pytorchOnce      sync.Once
	pytorchAvailable bool
)

// requirePyTorchは、PyTorchの参照値を作るPythonが無い環境でテストを飛ばす。
func requirePyTorch(t *testing.T) {
	t.Helper()
	pytorchOnce.Do(func() {
		pytorchAvailable = exec.Command("python", "-c", "import numpy, torch").Run() == nil
	})
	if !pytorchAvailable && os.Getenv("GOGRAD_REQUIRE_PYTORCH") != "1" {
		t.Skip("python with numpy and torch is unavailable")
	}
}
