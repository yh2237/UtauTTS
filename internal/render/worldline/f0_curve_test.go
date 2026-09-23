package worldline

import (
	"math"
	"strings"
	"testing"

	"utautts/internal/plan"
	"utautts/internal/render/base"
)

func TestWorldlineRejectsUnknownCVVCTimingBeforeResolvingAssets(t *testing.T) {
	_, err := renderWorldlineEngine(&plan.Plan{Units: []plan.Unit{{Role: "mora"}}}, base.Config{CVVCTiming: "unknown"}, "utautts-world-phrase")
	if err == nil || !strings.Contains(err.Error(), "unknown CVVC timing mode") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorldlineF0CurveInterpolatesInLogFrequency(t *testing.T) {
	p := &plan.Plan{Units: []plan.Unit{{NoteStartMS: 0}, {NoteStartMS: 100}}}
	curve := worldlineF0Curve(p, []float64{200, 400}, []float64{1, 1}, 220, 11)
	if math.Abs(curve[0]-200) > 0.01 || math.Abs(curve[10]-400) > 0.01 {
		t.Fatalf("curve endpoints = %.2f..%.2f", curve[0], curve[10])
	}
	if math.Abs(curve[5]-math.Sqrt(200*400)) > 0.1 {
		t.Fatalf("log midpoint = %.2f", curve[5])
	}
}

func TestWorldlineF0CurveOffsetIncludesPhraseLeading(t *testing.T) {
	p := &plan.Plan{Units: []plan.Unit{{NoteStartMS: 0}, {NoteStartMS: 100}}}
	curve := worldlineF0CurveAtOffset(p, []float64{200, 400}, []float64{1, 1}, 220, 13, 10, -20)
	if math.Abs(curve[0]-200) > 0.01 || math.Abs(curve[2]-200) > 0.01 {
		t.Fatalf("leading frames = %.2f, %.2f; want 200Hz", curve[0], curve[2])
	}
	if math.Abs(curve[12]-400) > 0.01 {
		t.Fatalf("second unit at shifted frame = %.2f, want 400Hz", curve[12])
	}
}

func TestWorldlineF0CurveAppliesLearnedPitchFactors(t *testing.T) {
	p := &plan.Plan{Units: []plan.Unit{{NoteStartMS: 0}, {NoteStartMS: 100}}}
	curve := worldlineF0Curve(p, []float64{200, 200}, []float64{1.03, 0.97}, 200, 11)
	if math.Abs(curve[0]-206) > 0.01 || math.Abs(curve[10]-194) > 0.01 {
		t.Fatalf("factored curve endpoints = %.2f..%.2f", curve[0], curve[10])
	}
}
