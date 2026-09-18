package plan

import (
	"fmt"
	"math"
	"strings"
	"unicode"
)

// UnitOverrideは1発話中の1原音だけに効く上書き値。oto.iniや原音そのものは変えない。
type UnitOverride struct {
	Index               int      `json:"unit_index"`
	OffsetMS            *float64 `json:"offset_ms,omitempty"`
	CutoffMS            *float64 `json:"cutoff_ms,omitempty"`
	ConsonantMS         *float64 `json:"consonant_ms,omitempty"`
	PreutteranceMS      *float64 `json:"preutterance_ms,omitempty"`
	OverlapMS           *float64 `json:"overlap_ms,omitempty"`
	PitchFactor         *float64 `json:"pitch_factor,omitempty"`
	EnergyFactor        *float64 `json:"energy_factor,omitempty"`
	ResamplerVelocity   *int     `json:"resampler_velocity,omitempty"`
	ResamplerVolume     *int     `json:"resampler_volume,omitempty"`
	ResamplerFlags      *string  `json:"resampler_flags,omitempty"`
	ResamplerModulation *int     `json:"resampler_modulation,omitempty"`
	ResamplerTempo      *float64 `json:"resampler_tempo,omitempty"`
}

// ApplyUnitOverridesは原音の確定後にユーザー編集値を当てはめる。
func ApplyUnitOverrides(units []Unit, overrides []UnitOverride) error {
	seen := make(map[int]struct{}, len(overrides))
	for _, override := range overrides {
		if override.Index < 0 || override.Index >= len(units) {
			return fmt.Errorf("unit override index %d is out of range", override.Index)
		}
		if _, exists := seen[override.Index]; exists {
			return fmt.Errorf("duplicate unit override index %d", override.Index)
		}
		seen[override.Index] = struct{}{}
		if err := validateUnitOverride(override); err != nil {
			return fmt.Errorf("unit override %d: %w", override.Index, err)
		}

		unit := &units[override.Index]
		if override.OffsetMS != nil {
			unit.OffsetMS = *override.OffsetMS
		}
		if override.CutoffMS != nil {
			unit.CutoffMS = *override.CutoffMS
		}
		if override.ConsonantMS != nil {
			unit.ConsonantMS = *override.ConsonantMS
		}
		if override.PreutteranceMS != nil {
			unit.PreutteranceMS = *override.PreutteranceMS
		}
		if override.OverlapMS != nil {
			unit.OverlapMS = *override.OverlapMS
		}
		if override.PitchFactor != nil {
			unit.PitchFactor = *override.PitchFactor
		}
		if override.EnergyFactor != nil {
			unit.EnergyFactor = *override.EnergyFactor
		}
		if override.ResamplerVelocity != nil {
			unit.ResamplerVelocity = *override.ResamplerVelocity
			unit.ResamplerVelocityOverride = true
		}
		if override.ResamplerVolume != nil {
			unit.ResamplerVolume = *override.ResamplerVolume
			unit.ResamplerVolumeOverride = true
		}
		if override.ResamplerFlags != nil {
			unit.ResamplerFlags = *override.ResamplerFlags
			unit.ResamplerFlagsOverride = true
		}
		if override.ResamplerModulation != nil {
			unit.ResamplerModulation = *override.ResamplerModulation
			unit.ResamplerModulationOverride = true
		}
		if override.ResamplerTempo != nil {
			unit.ResamplerTempo = *override.ResamplerTempo
			unit.ResamplerTempoOverride = true
		}
	}
	return nil
}

func validateUnitOverride(override UnitOverride) error {
	if override.OffsetMS != nil && !finite(*override.OffsetMS) {
		return fmt.Errorf("offset_ms must be finite")
	}
	if override.CutoffMS != nil && !finite(*override.CutoffMS) {
		return fmt.Errorf("cutoff_ms must be finite")
	}
	for name, value := range map[string]*float64{
		"consonant_ms":    override.ConsonantMS,
		"preutterance_ms": override.PreutteranceMS,
		"overlap_ms":      override.OverlapMS,
	} {
		if value != nil && (!finite(*value) || *value < 0) {
			return fmt.Errorf("%s must be finite and non-negative", name)
		}
	}
	for name, value := range map[string]*float64{
		"pitch_factor":  override.PitchFactor,
		"energy_factor": override.EnergyFactor,
	} {
		if value != nil && (!finite(*value) || *value <= 0) {
			return fmt.Errorf("%s must be finite and positive", name)
		}
	}
	if override.ResamplerVelocity != nil && (*override.ResamplerVelocity < 0 || *override.ResamplerVelocity > 200) {
		return fmt.Errorf("resampler_velocity must be between 0 and 200")
	}
	if override.ResamplerVolume != nil && (*override.ResamplerVolume < 0 || *override.ResamplerVolume > 200) {
		return fmt.Errorf("resampler_volume must be between 0 and 200")
	}
	if override.ResamplerModulation != nil && (*override.ResamplerModulation < 0 || *override.ResamplerModulation > 100) {
		return fmt.Errorf("resampler_modulation must be between 0 and 100")
	}
	if override.ResamplerTempo != nil && (!finite(*override.ResamplerTempo) || *override.ResamplerTempo <= 0 || *override.ResamplerTempo > 1000) {
		return fmt.Errorf("resampler_tempo must be finite and between 0 and 1000")
	}
	if override.ResamplerFlags != nil && strings.IndexFunc(*override.ResamplerFlags, unicode.IsSpace) >= 0 {
		return fmt.Errorf("resampler_flags must not contain whitespace")
	}
	return nil
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
