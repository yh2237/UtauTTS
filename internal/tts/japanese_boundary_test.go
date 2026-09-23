package tts

import (
	"math"
	"testing"

	"utautts/internal/render"
)

func flatPitchCurve(frames int) *render.PitchCurve {
	return &render.PitchCurve{FrameMS: 10, Cents: make([]float64, frames)}
}

// curveEndMSは曲線の最終フレーム時刻を返す。
func curveEndMS(curve *render.PitchCurve) float64 {
	return float64(len(curve.Cents)-1) * curve.FrameMS
}

func TestApplyBoundaryToneQuestionRisesAtEnd(t *testing.T) {
	curve := flatPitchCurve(30)
	result := applyBoundaryTone(curve, curveEndMS(curve), true, 1)
	if result == nil || len(result.Cents) != len(curve.Cents) {
		t.Fatalf("result = %#v", result)
	}
	last := result.Cents[len(result.Cents)-1]
	if math.Abs(last-boundaryToneRiseCents) > 1e-9 {
		t.Fatalf("question final cents = %.4f, want %.4f", last, boundaryToneRiseCents)
	}
	if result.Cents[0] != 0 {
		t.Fatalf("question start cents = %.4f, want 0", result.Cents[0])
	}
	if curve.Cents[len(curve.Cents)-1] != 0 {
		t.Fatalf("source curve was mutated: %.4f", curve.Cents[len(curve.Cents)-1])
	}
}

func TestApplyBoundaryToneStatementFallsAtEnd(t *testing.T) {
	curve := flatPitchCurve(30)
	result := applyBoundaryTone(curve, curveEndMS(curve), false, 1)
	last := result.Cents[len(result.Cents)-1]
	if math.Abs(last-boundaryToneFallCents) > 1e-9 {
		t.Fatalf("statement final cents = %.4f, want %.4f", last, boundaryToneFallCents)
	}
	if last >= 0 {
		t.Fatalf("statement final cents = %.4f, want negative", last)
	}
}

func TestApplyBoundaryToneRampsSmoothly(t *testing.T) {
	curve := flatPitchCurve(30)
	result := applyBoundaryTone(curve, curveEndMS(curve), true, 1)
	previous := 0.0
	for _, cents := range result.Cents {
		if cents < previous-1e-9 {
			t.Fatalf("boundary tone is not monotonic: %.4f after %.4f", cents, previous)
		}
		previous = cents
	}
}

func TestApplyBoundaryToneZeroOrNegativeStrengthIsIdentity(t *testing.T) {
	curve := flatPitchCurve(30)
	for _, strength := range []float64{0, -0.5, -3} {
		result := applyBoundaryTone(curve, curveEndMS(curve), true, strength)
		if result != curve {
			t.Fatalf("strength %.2f returned a new curve", strength)
		}
	}
}

func TestApplyBoundaryToneScalesDeviationByStrength(t *testing.T) {
	curve := flatPitchCurve(30)
	full := applyBoundaryTone(curve, curveEndMS(curve), true, 1)
	half := applyBoundaryTone(curve, curveEndMS(curve), true, 0.5)
	last := len(curve.Cents) - 1
	if math.Abs(half.Cents[last]*2-full.Cents[last]) > 1e-9 {
		t.Fatalf("half = %.4f, full = %.4f", half.Cents[last], full.Cents[last])
	}
}

func TestApplyBoundaryToneClampsStrength(t *testing.T) {
	curve := flatPitchCurve(30)
	over := applyBoundaryTone(curve, curveEndMS(curve), true, maxBoundaryToneStrength+1)
	capped := applyBoundaryTone(curve, curveEndMS(curve), true, maxBoundaryToneStrength)
	last := len(curve.Cents) - 1
	if over.Cents[last] != capped.Cents[last] {
		t.Fatalf("over strength = %.4f, capped = %.4f", over.Cents[last], capped.Cents[last])
	}
}

func TestApplyBoundaryToneKeepsFrameCountAndWindow(t *testing.T) {
	curve := flatPitchCurve(100)
	result := applyBoundaryTone(curve, curveEndMS(curve), true, 1)
	if len(result.Cents) != len(curve.Cents) {
		t.Fatalf("frame count = %d, want %d", len(result.Cents), len(curve.Cents))
	}
	// 窓の外側（終端から150msより前）は変わらない。
	for index := 0; index < 80; index++ {
		if result.Cents[index] != 0 {
			t.Fatalf("cents[%d] = %.4f, want 0", index, result.Cents[index])
		}
	}
}

func TestApplyBoundaryToneHoldsAfterEnd(t *testing.T) {
	curve := flatPitchCurve(40)
	result := applyBoundaryTone(curve, 200, true, 1)
	// 区間より後（末尾のポーズ区間）も最大値を保持して段差を作らない。
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

func TestBoundaryToneStrengthDefaults(t *testing.T) {
	if got := boundaryToneStrength(Config{}); got != 1 {
		t.Fatalf("zero strength = %.2f, want default 1", got)
	}
	if got := boundaryToneStrength(Config{BoundaryToneStrength: 0.5}); got != 0.5 {
		t.Fatalf("explicit strength = %.2f, want 0.5", got)
	}
}
