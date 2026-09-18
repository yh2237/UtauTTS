package plan

import (
	"math"
	"testing"
)

func TestApplyUnitOverrides(t *testing.T) {
	units := []Unit{{
		OffsetMS: 10, CutoffMS: 20, ConsonantMS: 30, PreutteranceMS: 40, OverlapMS: 5,
		PitchFactor: 1, EnergyFactor: 1,
	}}
	flags := "B0"
	tempo := 135.0
	if err := ApplyUnitOverrides(units, []UnitOverride{
		{
			Index:               0,
			OffsetMS:            float64Pointer(12),
			CutoffMS:            float64Pointer(-8),
			ConsonantMS:         float64Pointer(42),
			PreutteranceMS:      float64Pointer(55),
			OverlapMS:           float64Pointer(9),
			PitchFactor:         float64Pointer(1.2),
			EnergyFactor:        float64Pointer(.8),
			ResamplerVelocity:   intPointer(90),
			ResamplerVolume:     intPointer(110),
			ResamplerFlags:      &flags,
			ResamplerModulation: intPointer(20),
			ResamplerTempo:      &tempo,
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
	if unit.ResamplerVelocity != 90 || unit.ResamplerVolume != 110 ||
		unit.ResamplerFlags != flags || unit.ResamplerModulation != 20 ||
		unit.ResamplerTempo != tempo || !unit.ResamplerVelocityOverride ||
		!unit.ResamplerVolumeOverride || !unit.ResamplerFlagsOverride ||
		!unit.ResamplerModulationOverride || !unit.ResamplerTempoOverride {
		t.Fatalf("resampler override = %+v", unit)
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
