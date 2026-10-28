package hubert

import (
	"encoding/binary"
	"math"
	"os"
	"strconv"
	"testing"
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

// TestEncoderParity compares encoder output with saved external references.
// HUBERT_SAFETENSORS enables the fixed layer 0 and 6 fixtures.
// ENCODER_SAFETENSORS, ENCODER_REFERENCE and ENCODER_WAVE enable a custom
// reference; ENCODER_LAYER selects its layer (default 6).
func TestEncoderParity(t *testing.T) {
	layer := 6
	if value, err := strconv.Atoi(os.Getenv("ENCODER_LAYER")); err == nil {
		layer = value
	}
	type referenceCase struct {
		layer int
		path  string
	}
	for _, suite := range []struct {
		name          string
		modelPath     string
		wavePath      string
		references    []referenceCase
		fixedHuBERT   bool
		maxAbs        float64
		minCosine     float64
		missingConfig string
	}{
		{
			name:          "HuBERT",
			modelPath:     os.Getenv("HUBERT_SAFETENSORS"),
			wavePath:      "testdata/wave.f32",
			references:    []referenceCase{{0, "testdata/layer0.f32"}, {6, "testdata/layer6.f32"}},
			fixedHuBERT:   true,
			maxAbs:        float64(float32(5e-3)),
			missingConfig: "set HUBERT_SAFETENSORS",
		},
		{
			name:          "External",
			modelPath:     os.Getenv("ENCODER_SAFETENSORS"),
			wavePath:      os.Getenv("ENCODER_WAVE"),
			references:    []referenceCase{{layer, os.Getenv("ENCODER_REFERENCE")}},
			maxAbs:        1e-3,
			minCosine:     0.9999,
			missingConfig: "set ENCODER_SAFETENSORS, ENCODER_REFERENCE, ENCODER_WAVE",
		},
	} {
		t.Run(suite.name, func(t *testing.T) {
			if suite.modelPath == "" || suite.wavePath == "" || suite.references[0].path == "" {
				t.Skip(suite.missingConfig)
			}
			model, err := Load(suite.modelPath)
			if err != nil {
				t.Fatal(err)
			}
			defer model.Close()
			wave := readF32(t, suite.wavePath)
			dims := model.config.HiddenSize
			if suite.fixedHuBERT {
				dims = 768
			}
			for _, reference := range suite.references {
				t.Run("layer"+strconv.Itoa(reference.layer), func(t *testing.T) {
					got, err := model.Features(wave, reference.layer)
					if err != nil {
						t.Fatal(err)
					}
					want := readF32(t, reference.path)
					if len(want)%dims != 0 {
						t.Fatalf("reference has %d values, want a multiple of %d", len(want), dims)
					}
					frames := len(want) / dims
					if len(got) != frames {
						t.Fatalf("frames %d, want %d", len(got), frames)
					}
					var maxAbs, dot, normA, normB float64
					for frame, row := range got {
						if len(row) != dims {
							t.Fatalf("frame %d: dims %d, want %d", frame, len(row), dims)
						}
						for d, value := range row {
							a, b := float64(value), float64(want[frame*dims+d])
							difference := math.Abs(a - b)
							if suite.fixedHuBERT {
								// Preserve the fixture comparison's float32 subtraction.
								difference = math.Abs(float64(value - want[frame*dims+d]))
							}
							if difference > maxAbs {
								maxAbs = difference
							}
							dot += a * b
							normA += a * a
							normB += b * b
						}
					}
					cosine := dot / (math.Sqrt(normA) * math.Sqrt(normB))
					t.Logf("frames=%d dims=%d maxAbs=%.6g cosine=%.6f", frames, dims, maxAbs, cosine)
					if maxAbs > suite.maxAbs || (suite.minCosine != 0 && cosine < suite.minCosine) {
						t.Fatalf("encoder does not match the reference")
					}
				})
			}
		})
	}
}
