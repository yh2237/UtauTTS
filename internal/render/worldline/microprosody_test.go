package worldline

import (
	"math"
	"testing"

	"utautts/internal/plan"
)

func TestApplyMicroprosodyRaisesF0AfterVoicelessStop(t *testing.T) {
	synthesisPlan := &plan.Plan{Units: []plan.Unit{
		{Role: "mora", Mora: "か", NoteStartMS: 100},
		{Role: "mora", Mora: "あ", NoteStartMS: 300},
	}}
	curve := make([]float64, 50)
	for i := range curve {
		curve[i] = 200
	}
	applyMicroprosody(synthesisPlan, curve, 0, 10)
	if cents := 1200 * math.Log2(curve[10]/200); math.Abs(cents-62) > 0.01 {
		t.Fatalf("vowel onset after か = %.2f cents, want 62", cents)
	}
	if curve[30] != 200 || curve[29] != 200 || curve[32] != 200 {
		t.Fatalf("vowel-only mora changed: %v", curve[28:34])
	}
	if curve[7] != 200 || curve[15] != 200 {
		t.Fatalf("template leaked outside -20..+40 ms: %v %v", curve[7], curve[15])
	}
}

func TestApplyMicroprosodyKeepsUnvoicedFrames(t *testing.T) {
	synthesisPlan := &plan.Plan{Units: []plan.Unit{{Role: "mora", Mora: "た", NoteStartMS: 50}}}
	curve := make([]float64, 20)
	applyMicroprosody(synthesisPlan, curve, 0, 10)
	for i, value := range curve {
		if value != 0 {
			t.Fatalf("unvoiced frame %d became %v", i, value)
		}
	}
}
