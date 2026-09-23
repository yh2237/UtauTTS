package worldline

import (
	"reflect"
	"testing"

	"utautts/internal/plan"
	"utautts/internal/voicebank"
)

func TestWorldlineVCVTimingKeepsValidOTOAnchors(t *testing.T) {
	unit := plan.Unit{Role: "mora", AliasKind: "VCV", DurationMS: 140,
		PreutteranceMS: 210, OverlapMS: 70, ConsonantMS: 360,
		SpeechProfile: &voicebank.SpeechProfile{Applied: true, TrimmedLengthMS: 560, StableStartMS: 335}}
	plain := worldlineTiming(&plan.Plan{}, unit, 20)
	if plain.PreutteranceMS != 210 || plain.ConsonantMS != 360 || plain.OverlapMS != 70 {
		t.Fatalf("WORLD changed valid oto anchors: %+v", plain)
	}
	broken := unit
	broken.OverlapMS = 400
	got := worldlineTiming(&plan.Plan{}, broken, 20)
	if !got.CVApplied || got.OverlapMS > got.PreutteranceMS {
		t.Fatalf("WORLD did not repair a broken VCV boundary: %+v", got)
	}
}

func TestWorldlinePhoneTimingUnitsDoesNotMutatePlan(t *testing.T) {
	unit := plan.Unit{Role: "mora", AliasKind: "VCV", DurationMS: 140,
		PreutteranceMS: 210, OverlapMS: 70, ConsonantMS: 360}
	p := &plan.Plan{Units: []plan.Unit{unit}}
	got := worldlinePhoneTimingUnits(p, 20)
	if len(got) != 1 || !reflect.DeepEqual(got[0], unit) {
		t.Fatalf("WORLD default phone timing changed: %+v", got)
	}
	if !reflect.DeepEqual(p.Units[0], unit) {
		t.Fatalf("source plan was mutated: %+v", p.Units[0])
	}
}
