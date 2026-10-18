package hubert

import (
	"math"
	"os"
	"testing"
)

func TestRelativeBiasParity(t *testing.T) {
	modelPath := os.Getenv("WAVLM_SAFETENSORS")
	dumpPath := os.Getenv("WAVLM_BIAS")
	if modelPath == "" || dumpPath == "" {
		t.Skip("set WAVLM_SAFETENSORS and WAVLM_BIAS")
	}
	model, err := Load(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	defer model.Close()
	const frames = 74
	got, err := model.relativeBias(frames)
	if err != nil {
		t.Fatal(err)
	}
	want := readF32(t, dumpPath)
	heads := model.config.Heads
	if len(want) != heads*frames*frames {
		t.Fatalf("want %d values", len(want))
	}
	var maxAbs float64
	for h := 0; h < heads; h++ {
		for i := 0; i < frames; i++ {
			for j := 0; j < frames; j++ {
				a := float64(got[(i*frames+j)*heads+h])
				b := float64(want[(h*frames+i)*frames+j])
				if d := math.Abs(a - b); d > maxAbs {
					maxAbs = d
				}
			}
		}
	}
	t.Logf("max abs %.6g", maxAbs)
	if maxAbs > 1e-5 {
		t.Fatalf("relative bias mismatch: %.6g", maxAbs)
	}
}
