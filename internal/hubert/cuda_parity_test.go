//go:build windows

package hubert

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yh2237/gograd/cuda"
)

func TestFeaturesCUDA(t *testing.T) {
	if !cuda.Available() {
		t.Skip("CUDA is unavailable")
	}
	modelPath := filepath.Join("..", "..", "models", "english-reference", "hubert-base-ls960.safetensors")
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		t.Skip("HuBERT model is missing")
	}
	m, err := Load(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	// 同じ入力でCPUとCUDAの出力を比べる。入力は声に近い倍音を含む合成波にする。
	const sampleRate = 16000
	wave := make([]float32, sampleRate*3/2)
	for i := range wave {
		at := float64(i) / sampleRate
		wave[i] = float32(.35*math.Sin(2*math.Pi*140*at) + .2*math.Sin(2*math.Pi*280*at) + .1*math.Sin(2*math.Pi*560*at))
	}
	start := time.Now()
	cpu, err := m.Features(wave, 6)
	if err != nil {
		t.Fatal(err)
	}
	cpuTime := time.Since(start)
	start = time.Now()
	gpu, err := m.FeaturesCUDA(wave, 6)
	if err != nil {
		t.Fatal(err)
	}
	firstTime := time.Since(start)
	start = time.Now()
	gpu, err = m.FeaturesCUDA(wave, 6)
	if err != nil {
		t.Fatal(err)
	}
	gpuTime := time.Since(start)
	if len(cpu) != len(gpu) {
		t.Fatalf("frames: CPU %d, CUDA %d", len(cpu), len(gpu))
	}
	var maxAbs, cosineSum float64
	for i := range cpu {
		var dot, a2, b2 float64
		for j, a := range cpu[i] {
			b := gpu[i][j]
			maxAbs = math.Max(maxAbs, math.Abs(float64(a-b)))
			dot += float64(a) * float64(b)
			a2 += float64(a) * float64(a)
			b2 += float64(b) * float64(b)
		}
		cosineSum += dot / math.Sqrt(a2*b2)
	}
	cosine := cosineSum / float64(len(cpu))
	t.Logf("frames=%d max abs=%.8g mean cosine=%.8g CPU=%s CUDA first=%s CUDA cached=%s", len(cpu), maxAbs, cosine, cpuTime, firstTime, gpuTime)
	if maxAbs > 1e-2 || cosine < 0.999 {
		t.Fatalf("CPU/CUDA parity failed")
	}
}
