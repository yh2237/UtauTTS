package render

import (
	"fmt"
	"math"
	"testing"
)

func TestGPUWSOLAStaysCloseToCPU(t *testing.T) {
	if err := gpuWaveformAvailable(); err != nil {
		t.Skip(err)
	}
	for _, sampleRate := range []int{44100, 96000} {
		t.Run(fmt.Sprintf("%dHz", sampleRate), func(t *testing.T) {
			source := make([]float64, sampleRate/2)
			for frame := range source {
				time := float64(frame) / float64(sampleRate)
				source[frame] = 0.25*math.Sin(2*math.Pi*220*time) + 0.08*math.Sin(2*math.Pi*443*time)
			}
			cpu := wsola(source, sampleRate/5, sampleRate)
			gpu, err := gpuWSOLA(source, len(cpu), sampleRate)
			if err != nil {
				t.Fatal(err)
			}
			if len(gpu) != len(cpu) {
				t.Fatalf("GPU length=%d, want %d", len(gpu), len(cpu))
			}
			errorEnergy, signalEnergy := 0.0, 0.0
			for index := range cpu {
				delta := cpu[index] - gpu[index]
				errorEnergy += delta * delta
				signalEnergy += cpu[index] * cpu[index]
			}
			if relative := math.Sqrt(errorEnergy / math.Max(signalEnergy, 1e-12)); relative > 0.02 {
				t.Fatalf("GPU relative RMS error=%f, want <=0.02", relative)
			}
		})
	}
}
