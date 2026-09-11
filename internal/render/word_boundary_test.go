package render

import (
	"reflect"
	"testing"
	"utautts/internal/frontend"
	"utautts/internal/plan"
)

func TestWordEnvelopePreservesSupportAndSkipsPauses(t *testing.T) {
	p := &plan.Plan{Morae: []frontend.Mora{{WordIndex: 0}, {WordIndex: 1}, {WordIndex: 1}, {Pause: true}, {WordIndex: 2}}}
	points := []worldlineEnvelopePoint{{-40, 0}, {-20, 1}, {0, 1}, {100, 1}, {140, 0}}
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
