package world

import (
	"math"
	"strings"
	"testing"

	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/render/base"
)

func TestWorldRejectsUnknownCVVCTimingBeforeResolvingAssets(t *testing.T) {
	_, err := renderWorldEngine(&plan.Plan{Units: []plan.Unit{{Role: "mora"}}}, base.Config{CVVCTiming: "unknown"})
	if err == nil || !strings.Contains(err.Error(), "unknown CVVC timing mode") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorldF0CurveInterpolatesInLogFrequency(t *testing.T) {
	p := &plan.Plan{Units: []plan.Unit{{NoteStartMS: 0}, {NoteStartMS: 100}}}
	curve := worldF0CurveAtOffset(p, []float64{200, 400}, []float64{1, 1}, 220, 11, worldFrameMS, 0)
	if math.Abs(curve[0]-200) > 0.01 || math.Abs(curve[10]-400) > 0.01 {
		t.Fatalf("curve endpoints = %.2f..%.2f", curve[0], curve[10])
	}
	if math.Abs(curve[5]-math.Sqrt(200*400)) > 0.1 {
		t.Fatalf("log midpoint = %.2f", curve[5])
	}
}

func TestWorldF0CurveOffsetIncludesPhraseLeading(t *testing.T) {
	p := &plan.Plan{Units: []plan.Unit{{NoteStartMS: 0}, {NoteStartMS: 100}}}
	curve := worldF0CurveAtOffset(p, []float64{200, 400}, []float64{1, 1}, 220, 13, 10, -20)
	if math.Abs(curve[0]-200) > 0.01 || math.Abs(curve[2]-200) > 0.01 {
		t.Fatalf("leading frames = %.2f, %.2f; want 200Hz", curve[0], curve[2])
	}
	if math.Abs(curve[12]-400) > 0.01 {
		t.Fatalf("second unit at shifted frame = %.2f, want 400Hz", curve[12])
	}
}

func TestWorldF0CurveAppliesLearnedPitchFactors(t *testing.T) {
	p := &plan.Plan{Units: []plan.Unit{{NoteStartMS: 0}, {NoteStartMS: 100}}}
	curve := worldF0CurveAtOffset(p, []float64{200, 200}, []float64{1.03, 0.97}, 200, 11, worldFrameMS, 0)
	if math.Abs(curve[0]-206) > 0.01 || math.Abs(curve[10]-194) > 0.01 {
		t.Fatalf("factored curve endpoints = %.2f..%.2f", curve[0], curve[10])
	}
}
