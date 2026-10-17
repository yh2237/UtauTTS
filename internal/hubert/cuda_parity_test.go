//go:build windows

package hubert

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yh2237/UtauTTS/internal/audio"
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
	wavPath := filepath.Join("..", "..", "sample", "TETO-English-150401", "重音テト音声ライブラリー", "重音テト英語音源", "_3_h3_3-.wav")
	pcm, err := audio.ReadWav(wavPath)
	if err != nil {
		t.Fatal(err)
	}
	// Read the real recording as mono and resample it to HuBERT's 16 kHz input.
	mono := make([]float32, len(pcm.Data)/pcm.Channels)
	for i := range mono {
		for ch := 0; ch < pcm.Channels; ch++ {
			mono[i] += float32(pcm.Data[i*pcm.Channels+ch]) / (32768 * float32(pcm.Channels))
		}
	}
	wave := make([]float32, 24000)
	for i := range wave {
		at := float64(i) * float64(pcm.SampleRate) / 16000
		j := int(at)
		if j+1 >= len(mono) {
			t.Fatalf("recording shorter than 1.5 seconds")
		}
		frac := float32(at - float64(j))
		wave[i] = mono[j]*(1-frac) + mono[j+1]*frac
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
