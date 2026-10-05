package worldline

import (
	"reflect"
	"testing"

	"utautts/internal/plan"
	"utautts/internal/render/base"
)

func TestWorldlinePhoneTimingUnitsDoesNotMutatePlan(t *testing.T) {
	unit := plan.Unit{Role: "mora", AliasKind: "VCV", DurationMS: 140,
		PreutteranceMS: 210, OverlapMS: 70, ConsonantMS: 360}
	p := &plan.Plan{Units: []plan.Unit{unit}}
	got := base.NormalizedPhoneTimingUnits(p, 20)
	if len(got) != 1 || !reflect.DeepEqual(got[0], unit) {
		t.Fatalf("WORLD default phone timing changed: %+v", got)
	}
	if !reflect.DeepEqual(p.Units[0], unit) {
		t.Fatalf("source plan was mutated: %+v", p.Units[0])
	}
}
