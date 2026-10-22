package plan

import (
	"encoding/json"
	"math"
	"testing"
)

func TestApplyUnitOverrides(t *testing.T) {
	units := []Unit{{
		OffsetMS: 10, CutoffMS: 20, ConsonantMS: 30, PreutteranceMS: 40, OverlapMS: 5,
		PitchFactor: 1, EnergyFactor: 1,
	}}
	if err := ApplyUnitOverrides(units, []UnitOverride{
		{
			Index:             0,
			OffsetMS:          float64Pointer(12),
			CutoffMS:          float64Pointer(-8),
			ConsonantMS:       float64Pointer(42),
			PreutteranceMS:    float64Pointer(55),
			OverlapMS:         float64Pointer(9),
			PitchFactor:       float64Pointer(1.2),
			EnergyFactor:      float64Pointer(.8),
			ResamplerVolume:   intPointer(110),
			ResamplerVelocity: json.RawMessage(`90`),
		},
	}); err != nil {
		t.Fatal(err)
	}
	unit := units[0]
	if unit.OffsetMS != 12 || unit.CutoffMS != -8 || unit.ConsonantMS != 42 ||
		unit.PreutteranceMS != 55 || unit.OverlapMS != 9 ||
		unit.PitchFactor != 1.2 || unit.EnergyFactor != .8 {
		t.Fatalf("timing override = %+v", unit)
	}
	if unit.ResamplerVolume != 110 || !unit.ResamplerVolumeOverride {
		t.Fatalf("volume override = %+v", unit)
	}
}

func TestApplyUnitOverridesRejectsInvalidValues(t *testing.T) {
	if err := ApplyUnitOverrides([]Unit{{}}, []UnitOverride{{Index: 2}}); err == nil {
		t.Fatal("out-of-range unit override was accepted")
	}
	if err := ApplyUnitOverrides([]Unit{{}}, []UnitOverride{{
		Index: 0, OffsetMS: float64Pointer(math.Inf(1)),
	}}); err == nil {
		t.Fatal("non-finite offset was accepted")
	}
}

func float64Pointer(value float64) *float64 { return &value }

func intPointer(value int) *int { return &value }
