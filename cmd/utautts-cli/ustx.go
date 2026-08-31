package main

import (
	"math"

	"utautts/internal/frontend"
	"utautts/internal/openutau"
	"utautts/internal/plan"
	"utautts/internal/tts"
)

// ustxFrameCurves recomputes the frame-level intonation contour from the
// prosody model so the USTX export carries the smooth 10ms pitch curve.
// count is the number of utterances in the export (the CLI exports one).
func ustxFrameCurves(cfg tts.Config, count int) []openutau.FrameCurve {
	curves := make([]openutau.FrameCurve, count)
	if cfg.ProsodyModelPath == "" {
		return curves
	}
	predictConfig := cfg
	predictConfig.ApplyPitch = true
	if predictConfig.IntonationStrength <= 0 {
		predictConfig.IntonationStrength = 1
	}
	if predictConfig.Renderer == "" {
		predictConfig.Renderer = "waveform"
	}
	preview, err := tts.PredictProsody(predictConfig)
	if err != nil || preview == nil || preview.FramePitchCurve == nil {
		return curves
	}
	curves[0] = openutau.FrameCurve{
		FrameMS: preview.FramePitchCurve.FrameMS,
		Cents:   preview.FramePitchCurve.Cents,
	}
	return curves
}

// ustxProjectFromSynthesis builds a single-utterance UtauTTS project from a
// completed synthesis so the same parameters (reading, tone, mora pitch
// factors) can be exported to an OpenUtau USTX file via --ustx-out.
func ustxProjectFromSynthesis(cfg tts.Config, p *plan.Plan, voicebankID string) *openutau.UtauTTSProject {
	utterance := openutau.UtauTTSUtterance{
		Text:            cfg.Text,
		VoicebankID:     voicebankID,
		Tone:            p.Tone,
		MoraDurationMS:  cfg.MoraDurationMS,
		PauseDurationMS: cfg.PauseDurationMS,
		ApplyPitch:      cfg.ApplyPitch,
		Intonation:      cfg.IntonationStrength,
		AnalysisCache: openutau.UtauTTSAnalysisCache{
			Reading: p.Reading,
			Morae:   planMorae(p),
		},
	}
	// Per-mora pitch offsets (cents) derived from the applied pitch factors.
	// These are the model's mora-level values; they are only used as a
	// fallback when no frame contour is exported (the frame contour already
	// carries the model pitch, so it must not be added on top).
	cents := make([]float64, len(utterance.AnalysisCache.Morae))
	for _, unit := range p.Units {
		if unit.Silent || unit.Role == "transition" {
			continue
		}
		if unit.Position < 0 || unit.Position >= len(cents) {
			continue
		}
		factor := unit.PitchFactor
		if factor <= 0 {
			factor = 1
		}
		cents[unit.Position] = 1200 * math.Log2(factor)
	}
	utterance.AutomaticPitchPoints = cents
	// Actual synthesized timing: prosody predictions and duration overrides
	// replace the uniform mora duration during planning, so the exported
	// notes must land where the synthesis really placed them instead of on
	// the fixed MoraDurationMS grid.
	morae := utterance.AnalysisCache.Morae
	durationsMS := make([]float64, len(morae))
	positionsMS := make([]float64, len(morae))
	for _, unit := range p.Units {
		if unit.Silent || unit.Role == "transition" {
			continue
		}
		if unit.Position < 0 || unit.Position >= len(morae) {
			continue
		}
		positionsMS[unit.Position] = unit.NoteStartMS
		if unit.DurationMS > 0 {
			durationsMS[unit.Position] = unit.DurationMS
		}
	}
	utterance.AutomaticMoraDurMS = durationsMS
	utterance.AutomaticMoraPosMS = positionsMS
	return &openutau.UtauTTSProject{
		Format:        "utautts-project",
		FormatVersion: 5,
		Utterances:    []openutau.UtauTTSUtterance{utterance},
	}
}

// planMorae reconstructs the mora sequence (including pauses) from the
// reading with the same parser the plan uses, so long vowel marks carry the
// correct vowel for USTX extension-note export.
func planMorae(p *plan.Plan) []openutau.UtauTTSMora {
	morae, err := frontend.ParseKana(p.Reading)
	if err == nil && len(morae) > 0 {
		result := make([]openutau.UtauTTSMora, len(morae))
		for index, mora := range morae {
			result[index] = openutau.UtauTTSMora{
				Position: index, Mora: mora.Text, Pause: mora.Pause,
				Consonant: mora.Consonant, Vowel: mora.Vowel,
			}
		}
		return result
	}
	// Fallback: derive from the plan units (no vowel information).
	var fallback []openutau.UtauTTSMora
	seen := make(map[int]bool)
	for _, unit := range p.Units {
		if seen[unit.Position] {
			continue
		}
		seen[unit.Position] = true
		if unit.Silent {
			fallback = append(fallback, openutau.UtauTTSMora{Position: unit.Position, Pause: true})
			continue
		}
		fallback = append(fallback, openutau.UtauTTSMora{Position: unit.Position, Mora: unit.Mora})
	}
	return fallback
}
