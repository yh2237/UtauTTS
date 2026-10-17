package hubert

import (
	"encoding/binary"
	"math"
	"os"
	"sync"
	"testing"
	"time"
)

func readF32(t *testing.T, path string) []float32 {
	t.Helper()
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if len(b)%4 != 0 {
		t.Fatal("unaligned fixture")
	}
	x := make([]float32, len(b)/4)
	for i := range x {
		x[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return x
}

func TestGELUApproximation(t *testing.T) {
	var maxAbs float64
	for index := -80000; index <= 80000; index++ {
		value := float32(index) / 10000
		want := float64(value) * .5 * (1 + math.Erf(float64(value)/math.Sqrt2))
		difference := math.Abs(float64(gelu(value)) - want)
		if difference > maxAbs {
			maxAbs = difference
		}
	}
	if maxAbs > 2e-6 {
		t.Fatalf("GELU max abs %.6g", maxAbs)
	}
}
func TestParity(t *testing.T) {
	path := os.Getenv("HUBERT_SAFETENSORS")
	if path == "" {
		t.Skip("set HUBERT_SAFETENSORS")
	}
	m, e := Load(path)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	wave := readF32(t, "testdata/wave.f32")
	for _, layer := range []int{0, 6} {
		start := time.Now()
		got, e := m.Features(wave, layer)
		if e != nil {
			t.Fatal(e)
		}
		want := readF32(t, "testdata/layer"+string(rune('0'+layer))+".f32")
		if len(got)*768 != len(want) {
			t.Fatalf("layer %d: got %d frames want %d", layer, len(got), len(want)/768)
		}
		var maxAbs float32
		for i, row := range got {
			for j, v := range row {
				d := float32(math.Abs(float64(v - want[i*768+j])))
				if d > maxAbs {
					maxAbs = d
				}
			}
		}
		t.Logf("layer %d max abs %.6g time %s", layer, maxAbs, time.Since(start))
		if maxAbs > 5e-3 {
			t.Errorf("layer %d parity", layer)
		}
	}
}
func TestTenSecondTiming(t *testing.T) {
	path := os.Getenv("HUBERT_SAFETENSORS")
	if path == "" {
		t.Skip("set HUBERT_SAFETENSORS")
	}
	m, e := Load(path)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	wave := make([]float32, 160000)
	for i := range wave {
		v := float64(i)
		wave[i] = float32(.13*math.Sin(v*.017) + .07*math.Sin(v*.041) + .01*math.Cos(v*.003))
	}
	start := time.Now()
	features, e := m.Features(wave, 6)
	if e != nil {
		t.Fatal(e)
	}
	t.Logf("10 s audio: %d frames, %s", len(features), time.Since(start))
}

func TestConcurrentFeaturesMatchSequential(t *testing.T) {
	path := os.Getenv("HUBERT_SAFETENSORS")
	if path == "" {
		t.Skip("set HUBERT_SAFETENSORS")
	}
	m, e := Load(path)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	waves := make([][]float32, 4)
	for w := range waves {
		waves[w] = make([]float32, 16000+w*4000)
		for i := range waves[w] {
			v := float64(i)
			waves[w][i] = float32(.1*math.Sin(v*(.011+.003*float64(w))) + .02*math.Cos(v*.002))
		}
	}
	results := make([][][]float32, len(waves))
	var wait sync.WaitGroup
	for w := range waves {
		wait.Go(func() {
			var err error
			if results[w], err = m.Features(waves[w], 6); err != nil {
				t.Error(err)
			}
		})
	}
	wait.Wait()
	for w, wave := range waves {
		want, e := m.Features(wave, 6)
		if e != nil {
			t.Fatal(e)
		}
		if len(results[w]) != len(want) {
			t.Fatalf("wave %d: %d frames, want %d", w, len(results[w]), len(want))
		}
		for i := range want {
			for j := range want[i] {
				if results[w][i][j] != want[i][j] {
					t.Fatalf("wave %d differs at frame %d", w, i)
				}
			}
		}
	}
}
