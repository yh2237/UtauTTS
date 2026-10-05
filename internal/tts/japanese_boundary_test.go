package tts

import (
	"math"
	"testing"

	"utautts/internal/render"
)

func flatPitchCurve(frames int) *render.PitchCurve {
	return &render.PitchCurve{FrameMS: 10, Cents: make([]float64, frames)}
}

func curveEndMS(curve *render.PitchCurve) float64 {
	return float64(len(curve.Cents)-1) * curve.FrameMS
}

func TestApplyBoundaryToneShapes(t *testing.T) {
	curve := flatPitchCurve(30)
	end := curveEndMS(curve)
	last := len(curve.Cents) - 1

	t.Run("question rises at end", func(t *testing.T) {
		result := applyBoundaryTone(curve, end, true, 1)
		if result == nil || len(result.Cents) != len(curve.Cents) {
			t.Fatalf("result = %#v", result)
		}
		if math.Abs(result.Cents[last]-boundaryToneRiseCents) > 1e-9 {
			t.Fatalf("question final cents = %.4f, want %.4f", result.Cents[last], boundaryToneRiseCents)
		}
		if result.Cents[0] != 0 {
			t.Fatalf("question start cents = %.4f, want 0", result.Cents[0])
		}
		if curve.Cents[last] != 0 {
			t.Fatalf("source curve was mutated: %.4f", curve.Cents[last])
		}
	})
	t.Run("statement falls at end", func(t *testing.T) {
		got := applyBoundaryTone(curve, end, false, 1).Cents[last]
		if math.Abs(got-boundaryToneFallCents) > 1e-9 || got >= 0 {
			t.Fatalf("statement final cents = %.4f, want %.4f", got, boundaryToneFallCents)
		}
	})
	t.Run("ramps smoothly", func(t *testing.T) {
		previous := 0.0
		for _, cents := range applyBoundaryTone(curve, end, true, 1).Cents {
			if cents < previous-1e-9 {
				t.Fatalf("boundary tone is not monotonic: %.4f after %.4f", cents, previous)
			}
			previous = cents
		}
	})
	t.Run("zero or negative strength is identity", func(t *testing.T) {
		for _, strength := range []float64{0, -0.5, -3} {
			if result := applyBoundaryTone(curve, end, true, strength); result != curve {
				t.Fatalf("strength %.2f returned a new curve", strength)
			}
		}
	})
	t.Run("scales deviation by strength", func(t *testing.T) {
		full := applyBoundaryTone(curve, end, true, 1)
		half := applyBoundaryTone(curve, end, true, 0.5)
		if math.Abs(half.Cents[last]*2-full.Cents[last]) > 1e-9 {
			t.Fatalf("half = %.4f, full = %.4f", half.Cents[last], full.Cents[last])
		}
	})
	t.Run("clamps strength", func(t *testing.T) {
		over := applyBoundaryTone(curve, end, true, maxBoundaryToneStrength+1)
		capped := applyBoundaryTone(curve, end, true, maxBoundaryToneStrength)
		if over.Cents[last] != capped.Cents[last] {
			t.Fatalf("over strength = %.4f, capped = %.4f", over.Cents[last], capped.Cents[last])
		}
	})
}

func TestApplyBoundaryToneKeepsFrameCountAndWindow(t *testing.T) {
	curve := flatPitchCurve(100)
	result := applyBoundaryTone(curve, curveEndMS(curve), true, 1)
	if len(result.Cents) != len(curve.Cents) {
		t.Fatalf("frame count = %d, want %d", len(result.Cents), len(curve.Cents))
	}
	for index := 0; index < 80; index++ {
		if result.Cents[index] != 0 {
			t.Fatalf("cents[%d] = %.4f, want 0", index, result.Cents[index])
		}
	}
}

func TestApplyBoundaryToneHoldsAfterEnd(t *testing.T) {
	curve := flatPitchCurve(40)
	result := applyBoundaryTone(curve, 200, true, 1)
	for index := 21; index < len(result.Cents); index++ {
		if math.Abs(result.Cents[index]-boundaryToneRiseCents) > 1e-9 {
			t.Fatalf("cents[%d] = %.4f, want held %.4f", index, result.Cents[index], boundaryToneRiseCents)
		}
	}
}

func TestApplyBoundaryToneShortCurveUsesWholeWindow(t *testing.T) {
	curve := flatPitchCurve(6)
	result := applyBoundaryTone(curve, 50, true, 1)
	last := result.Cents[len(result.Cents)-1]
	if math.Abs(last-boundaryToneRiseCents) > 1e-9 {
		t.Fatalf("short curve final cents = %.4f, want %.4f", last, boundaryToneRiseCents)
	}
}

func TestApplyBoundaryToneNilIsSafe(t *testing.T) {
	if got := applyBoundaryTone(nil, 300, true, 1); got != nil {
		t.Fatalf("nil curve = %#v, want nil", got)
	}
	empty := &render.PitchCurve{FrameMS: 10}
	if got := applyBoundaryTone(empty, 300, true, 1); got != empty {
		t.Fatalf("empty curve = %#v, want identity", got)
	}
}

func TestBoundaryToneEnabledDefaultsOn(t *testing.T) {
	if !boundaryToneEnabled(Config{}) {
		t.Fatal("nil BoundaryTone should default to enabled")
	}
	disabled := false
	if boundaryToneEnabled(Config{BoundaryTone: &disabled}) {
		t.Fatal("explicit false should disable boundary tone")
	}
}
