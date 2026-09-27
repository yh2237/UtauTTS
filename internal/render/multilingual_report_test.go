package render

import (
	"testing"
	"utautts/internal/plan"
)

func TestSpeechAnchorsSurviveRenderReportWithoutSharing(t *testing.T) {
	p := &plan.Plan{Units: []plan.Unit{{SpeechMapping: "oto-landmark-prior-v1", SpeechSourceAnchorsMS: []float64{0, 100, 300}, SpeechTargetAnchorsMS: []float64{0, 40, 200}}}}
	r := reportFromPlan("utautts-world-phrase", p)
	out := &plan.Plan{Units: make([]plan.Unit, 1)}
	r.ApplyTo(out)
	if out.Units[0].SpeechMapping != "oto-landmark-prior-v1" || out.Units[0].SpeechTargetAnchorsMS[1] != 40 {
		t.Fatal(out)
	}
	out.Units[0].SpeechSourceAnchorsMS[1] = 999
	if p.Units[0].SpeechSourceAnchorsMS[1] != 100 || r.Units[0].SpeechSourceAnchorsMS[1] != 100 {
		t.Fatal("shared diagnostic slices")
	}
}
