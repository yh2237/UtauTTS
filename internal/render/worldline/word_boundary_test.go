package worldline

import (
	"reflect"
	"testing"

	"utautts/internal/frontend"
	"utautts/internal/plan"
	"utautts/internal/render/base"
)

func TestWordEnvelopePreservesSupportAndSkipsPauses(t *testing.T) {
	p := &plan.Plan{Morae: []frontend.Mora{{WordIndex: 0}, {WordIndex: 1}, {WordIndex: 1}, {Pause: true}, {WordIndex: 2}}}
	points := []base.WorldlineEnvelopePoint{{XMS: -40, Y: 0}, {XMS: -20, Y: 1}, {XMS: 0, Y: 1}, {XMS: 100, Y: 1}, {XMS: 140, Y: 0}}
	got, mark := wordBoundaryEnvelope(p, plan.Unit{Position: 1, Role: "mora"}, points)
	if mark == "" || got[1].XMS != -30 || got[3] != points[3] {
		t.Fatal(got, mark)
	}
	if got[0] != points[0] || got[4] != points[4] || points[1].XMS != -20 {
		t.Fatal("support or input changed")
	}
	for _, pos := range []int{2, 3, 4} {
		g, m := wordBoundaryEnvelope(p, plan.Unit{Position: pos}, points)
		if m != "" || !reflect.DeepEqual(g, points) {
			t.Fatal("pause/intraword changed", pos, g)
		}
	}
	got, mark = wordBoundaryEnvelope(p, plan.Unit{Position: 0}, points)
	if mark == "" || got[3].XMS != 120 || got[1] != points[1] {
		t.Fatal(got, mark)
	}
}
