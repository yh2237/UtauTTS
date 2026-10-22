//go:build windows

package hubert

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yh2237/gograd/tensor"
)

var (
	hubertModelPath = filepath.Join("..", "..", "models", "reference", "hubert-base-ls960.safetensors")
	wavlmModelPath  = filepath.Join("..", "..", "models", "reference", "wavlm", "wavlm-base-plus.safetensors")
)

func TestFeaturesCUDA(t *testing.T) {
	checkGPUParity(t, tensor.CUDA, hubertModelPath)
}

func TestFeaturesCUDAWavLM(t *testing.T) {
	checkGPUParity(t, tensor.CUDA, wavlmModelPath)
}

func TestFeaturesDirectML(t *testing.T) {
	checkGPUParity(t, tensor.DirectML, wavlmModelPath)
}

func TestFeaturesVulkan(t *testing.T) {
	checkGPUParity(t, tensor.Vulkan, wavlmModelPath)
}

// TestLastDeviceは、特徴量の計算に使ったデバイスが記録されることを確かめる。
func TestLastDevice(t *testing.T) {
	if _, err := os.Stat(wavlmModelPath); os.IsNotExist(err) {
		t.Skipf("%s is missing", wavlmModelPath)
	}
	m, err := Load(wavlmModelPath)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	wave := make([]float32, 16000)
	for i := range wave {
		wave[i] = float32(math.Sin(2 * math.Pi * 140 * float64(i) / 16000))
	}
	if _, err := m.Features(wave, 1); err != nil {
		t.Fatal(err)
	}
	if got := LastDevice(); got != "CPU" {
		t.Fatalf("Features used %q, want CPU", got)
	}
	if _, err := m.FeaturesAccelerated(wave, 1); err != nil {
		t.Fatal(err)
	}
	if got := LastDevice(); got == "" {
		t.Fatal("FeaturesAccelerated did not record a device")
	} else {
		t.Logf("FeaturesAccelerated used %s", got)
	}
}

// checkGPUParityは、同じ入力でCPUとGPUの出力を比べる。入力は声に近い倍音を含む合成波にする。
func checkGPUParity(t *testing.T, device tensor.Device, modelPath string) {
	t.Helper()
	if _, err := enterGPUDevice(device); err != nil {
		t.Skipf("%s is unavailable: %v", device, err)
	}
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		t.Skipf("%s is missing", modelPath)
	}
	m, err := Load(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
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
	gpu, err := m.featuresOnDevice(device, wave, 6, nil)
	if err != nil {
		t.Fatal(err)
	}
	firstTime := time.Since(start)
	start = time.Now()
	gpu, err = m.featuresOnDevice(device, wave, 6, nil)
	if err != nil {
		t.Fatal(err)
	}
	gpuTime := time.Since(start)
	if len(cpu) != len(gpu) {
		t.Fatalf("frames: CPU %d, GPU %d", len(cpu), len(gpu))
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
	t.Logf("device=%s frames=%d max abs=%.8g mean cosine=%.8g CPU=%s GPU first=%s GPU cached=%s", device, len(cpu), maxAbs, cosine, cpuTime, firstTime, gpuTime)
	if maxAbs > 1e-2 || cosine < 0.999 {
		t.Fatalf("CPU/GPU parity failed on %s", device)
	}
}
