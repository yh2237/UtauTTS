package native

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"utautts/internal/synth"
)

type synthesizeRequest struct {
	synth.Request
	OutputPath string `json:"output_path"`
}

func synthesisUnits(result *synth.Result) []map[string]any {
	if result == nil {
		return nil
	}
	synthesisPlan := result.RenderedPlan()
	if synthesisPlan == nil {
		return nil
	}
	units := make([]map[string]any, 0, len(synthesisPlan.Units))
	for index, unit := range synthesisPlan.Units {
		renderStart := unit.NoteStartMS - math.Max(0, unit.EffectivePreutteranceMS) + synthesisPlan.LeadingMarginMS
		units = append(units, map[string]any{
			"unit_index":                    index,
			"position":                      unit.Position,
			"role":                          unit.Role,
			"mora":                          unit.Mora,
			"alias":                         unit.Alias,
			"source":                        unit.Source,
			"source_name":                   filepath.Base(unit.Source),
			"silent":                        unit.Silent,
			"note_start_ms":                 unit.NoteStartMS,
			"render_start_ms":               renderStart,
			"duration_ms":                   unit.DurationMS,
			"offset_ms":                     unit.OffsetMS,
			"consonant_ms":                  unit.ConsonantMS,
			"cutoff_ms":                     unit.CutoffMS,
			"preutterance_ms":               unit.PreutteranceMS,
			"overlap_ms":                    unit.OverlapMS,
			"effective_preutterance_ms":     unit.EffectivePreutteranceMS,
			"effective_consonant_ms":        unit.EffectiveConsonantMS,
			"effective_overlap_ms":          unit.EffectiveOverlapMS,
			"pitch_factor":                  unit.PitchFactor,
			"energy_factor":                 unit.EnergyFactor,
			"resampler_velocity":            unit.ResamplerVelocity,
			"resampler_volume":              unit.ResamplerVolume,
			"resampler_flags":               unit.ResamplerFlags,
			"resampler_modulation":          unit.ResamplerModulation,
			"resampler_tempo":               unit.ResamplerTempo,
			"resampler_velocity_override":   unit.ResamplerVelocityOverride,
			"resampler_volume_override":     unit.ResamplerVolumeOverride,
			"resampler_flags_override":      unit.ResamplerFlagsOverride,
			"resampler_modulation_override": unit.ResamplerModulationOverride,
			"resampler_tempo_override":      unit.ResamplerTempoOverride,
		})
	}
	return units
}

func pcmPeaks(data []int16, channels int, maximumPoints int) (mins, maxs []float64) {
	if channels <= 0 || len(data) == 0 {
		return nil, nil
	}
	frames := len(data) / channels
	if frames <= 0 {
		return nil, nil
	}
	points := frames
	if maximumPoints > 0 && points > maximumPoints {
		points = maximumPoints
	}
	mins = make([]float64, points)
	maxs = make([]float64, points)
	for point := 0; point < points; point++ {
		start := point * frames / points
		end := (point + 1) * frames / points
		if end <= start {
			end = start + 1
		}
		minimum, maximum := 1.0, -1.0
		for frame := start; frame < end && frame < frames; frame++ {
			for channel := 0; channel < channels; channel++ {
				sample := float64(data[frame*channels+channel]) / 32768.0
				if sample < minimum {
					minimum = sample
				}
				if sample > maximum {
					maximum = sample
				}
			}
		}
		mins[point] = minimum
		maxs[point] = maximum
	}
	return mins, maxs
}

func waveformPeaks(result *synth.Result, maximumPoints int) (mins, maxs []float64) {
	if result == nil || result.Audio == nil {
		return nil, nil
	}
	return pcmPeaks(result.Audio.Data, result.Audio.Channels, maximumPoints)
}

func (e *Engine) synthesize(data []byte) (any, error) {
	request := synthesizeRequest{Request: synth.DefaultRequest()}
	if err := json.Unmarshal(data, &request); err != nil {
		return nil, fmt.Errorf("decode synthesis request: %w", err)
	}
	if request.Text == "" && request.ReadingOrKana() == "" {
		return nil, fmt.Errorf("text or reading is required")
	}
	if request.OutputPath == "" {
		return nil, fmt.Errorf("output_path is required")
	}
	result, err := e.synth.SynthesizeContext(e.ctx, request.Request.Normalized())
	if err != nil {
		return nil, err
	}
	outputPath, err := filepath.Abs(request.OutputPath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return nil, err
	}
	if err := synth.WriteFiles(outputPath, result, synth.ExportOptions{}); err != nil {
		return nil, err
	}
	renderedPlan := result.RenderedPlan()
	leadingMarginMS := 0.0
	if renderedPlan != nil {
		leadingMarginMS = renderedPlan.LeadingMarginMS
	}
	waveformMin, waveformMax := waveformPeaks(result, 2400)
	return map[string]any{
		"output_path":           outputPath,
		"reading":               result.Plan.Reading,
		"duration_ms":           result.DurationMS,
		"leading_margin_ms":     leadingMarginMS,
		"lab":                   result.Lab,
		"unit_count":            len(result.Plan.Units),
		"engine":                result.RendererID,
		"mora_durations_ms":     result.MoraDurationsMS,
		"mora_positions_ms":     result.MoraPositionsMS,
		"pitch_points":          result.PitchPoints,
		"units":                 synthesisUnits(result),
		"waveform_min":          waveformMin,
		"waveform_max":          waveformMax,
		"waveform_sample_rate":  result.Audio.SampleRate,
		"prosody_model_applied": e.synth.ModelAvailable(request.ModelID),
	}, nil
}
