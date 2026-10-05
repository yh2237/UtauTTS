package tts

import (
	"fmt"
	"math"
)

func validateConfig(cfg Config) error {
	finite := map[string]float64{
		"mora_duration_ms":          cfg.MoraDurationMS,
		"pause_duration_ms":         cfg.PauseDurationMS,
		"release_ms":                cfg.ReleaseMS,
		"intonation_strength":       cfg.IntonationStrength,
		"context_duration_strength": cfg.ContextDurationStrength,
		"boundary_tone_strength":    cfg.BoundaryToneStrength,
		"stretch_adapt_strength":    cfg.StretchAdaptStrength,
		"pause_context_strength":    cfg.PauseContextStrength,
		"boundary_bridge_ms":        cfg.BoundaryBridgeMS,
		"boundary_bridge_threshold": cfg.BoundaryBridgeThreshold,
	}
	for name, value := range finite {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("%s must be finite, got %v", name, value)
		}
	}
	for index, factor := range cfg.PitchFactors {
		if math.IsNaN(factor) || math.IsInf(factor, 0) {
			return fmt.Errorf("pitch factors: value %d must be finite, got %v", index, factor)
		}
	}
	for index, duration := range cfg.MoraDurationsMS {
		if math.IsNaN(duration) || math.IsInf(duration, 0) {
			return fmt.Errorf("mora durations: value %d must be finite, got %v", index, duration)
		}
	}
	if cfg.IntonationStrength < 0 || cfg.IntonationStrength > MaxIntonationStrength {
		return fmt.Errorf("intonation_strength must be between 0 and %.0f, got %v", MaxIntonationStrength, cfg.IntonationStrength)
	}
	if cfg.ReleaseMS < 0 {
		return fmt.Errorf("release_ms must be non-negative, got %v", cfg.ReleaseMS)
	}
	return nil
}
