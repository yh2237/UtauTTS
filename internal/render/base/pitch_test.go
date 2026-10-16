package base

import (
	"math"
	"testing"

	"github.com/yh2237/UtauTTS/internal/plan"
)

func TestApplyTonePitchMovesReferenceToTone(t *testing.T) {
	factors := []float64{1, 1.5}
	ApplyTonePitch(&plan.Plan{Tone: "A4"}, factors, 220)
	if math.Abs(factors[0]-2) > 1e-9 || math.Abs(factors[1]-3) > 1e-9 {
		t.Fatalf("factors = %v", factors)
	}
	factors = []float64{1, 1.5}
	ApplyTonePitch(&plan.Plan{}, factors, 220)
	if factors[0] != 1 || factors[1] != 1.5 {
		t.Fatalf("unspecified tone changed factors: %v", factors)
	}
}
