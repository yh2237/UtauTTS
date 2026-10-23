package render

import (
	"errors"
	"fmt"
	"math"

	"github.com/yh2237/UtauTTS/internal/audio"
	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/render/base"
)

func renderMutable(synthesisPlan *plan.Plan, cfg Config) (*audio.PCM, error) {
	if err := base.ContextError(cfg.Context); err != nil {
		return nil, err
	}
	for name, value := range map[string]float64{
		"release_ms":              cfg.ReleaseMS,
		"leading_preutterance_ms": cfg.LeadingPreutteranceMS,
		"intonation_strength":     cfg.IntonationStrength,
	} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, fmt.Errorf("%s must be finite, got %v", name, value)
		}
	}
	if cfg.ReleaseMS < 0 {
		return nil, fmt.Errorf("release_ms must be non-negative, got %v", cfg.ReleaseMS)
	}
	if cfg.LeadingPreutteranceMS < 0 {
		return nil, fmt.Errorf("leading_preutterance_ms must be non-negative, got %v", cfg.LeadingPreutteranceMS)
	}
	if !cfg.ReleaseSet && cfg.ReleaseMS == 0 {
		cfg.ReleaseMS = DefaultReleaseMS
	}
	if cfg.IntonationStrength < 0 || cfg.IntonationStrength > MaxIntonationStrength {
		return nil, fmt.Errorf("intonation_strength must be between 0 and %.0f, got %v", MaxIntonationStrength, cfg.IntonationStrength)
	}
	if !cfg.ApplyPitch {
		cfg.IntonationStrength = 0
		cfg.PitchCurve = nil
	}
	if cfg.PitchCurve != nil {
		if cfg.PitchCurve.FrameMS < 0.1 || math.IsNaN(cfg.PitchCurve.FrameMS) || math.IsInf(cfg.PitchCurve.FrameMS, 0) || len(cfg.PitchCurve.Cents) == 0 {
			return nil, errors.New("pitch curve requires frame_ms >= 0.1 and at least one value")
		}
		for index, cents := range cfg.PitchCurve.Cents {
			if math.IsNaN(cents) || math.IsInf(cents, 0) {
				return nil, fmt.Errorf("pitch curve value %d is not finite", index)
			}
			if math.Abs(cents) > 4800 {
				return nil, fmt.Errorf("pitch curve value %d is outside the supported +/-4800 cent range", index)
			}
		}
	}
	if cfg.Backend == "" {
		cfg.Backend = base.DefaultBackend
	}
	if backend, ok := base.RendererImplementation(cfg.Backend); ok {
		return backend(synthesisPlan, cfg)
	}
	return nil, fmt.Errorf("unknown renderer backend %q", cfg.Backend)
}
