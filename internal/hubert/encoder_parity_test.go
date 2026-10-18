package hubert

import (
	"math"
	"os"
	"strconv"
	"testing"
)

// TestEncoderParityは、Goのエンコーダの出力を、transformersで保存した参照と比べる。
// ENCODER_SAFETENSORS、ENCODER_REFERENCE、ENCODER_WAVEを設定したときだけ動く。
// ENCODER_LAYERで層を選べる（既定は6）。
func TestEncoderParity(t *testing.T) {
	modelPath := os.Getenv("ENCODER_SAFETENSORS")
	referencePath := os.Getenv("ENCODER_REFERENCE")
	wavePath := os.Getenv("ENCODER_WAVE")
	if modelPath == "" || referencePath == "" || wavePath == "" {
		t.Skip("set ENCODER_SAFETENSORS, ENCODER_REFERENCE, ENCODER_WAVE")
	}
	layer := 6
	if value, err := strconv.Atoi(os.Getenv("ENCODER_LAYER")); err == nil {
		layer = value
	}
	model, err := Load(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	defer model.Close()
	got, err := model.Features(readF32(t, wavePath), layer)
	if err != nil {
		t.Fatal(err)
	}
	want := readF32(t, referencePath)
	dims := model.config.HiddenSize
	if len(want)%dims != 0 {
		t.Fatalf("reference has %d values, want a multiple of %d", len(want), dims)
	}
	frames := len(want) / dims
	if len(got) != frames {
		t.Fatalf("frames %d, want %d", len(got), frames)
	}
	var maxAbs, dot, normA, normB float64
	for t := 0; t < frames; t++ {
		for d := 0; d < dims; d++ {
			a, b := float64(got[t][d]), float64(want[t*dims+d])
			if difference := math.Abs(a - b); difference > maxAbs {
				maxAbs = difference
			}
			dot += a * b
			normA += a * a
			normB += b * b
		}
	}
	cosine := dot / (math.Sqrt(normA) * math.Sqrt(normB))
	t.Logf("frames=%d dims=%d maxAbs=%.6g cosine=%.6f", frames, dims, maxAbs, cosine)
	if maxAbs > 1e-3 || cosine < 0.9999 {
		t.Fatalf("encoder does not match the reference")
	}
}
