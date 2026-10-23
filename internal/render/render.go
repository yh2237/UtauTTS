package render

import (
	"errors"
	"fmt"
	"math"

	"github.com/yh2237/UtauTTS/internal/audio"
	"github.com/yh2237/UtauTTS/internal/engine"
	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/render/base"
)

func init() {
	base.RegisterRenderer(engine.TestConcatProvider, renderTestConcat)
}

func IsKnownRenderer(id string) bool {
	if id == "" {
		return true
	}
	return engine.BuiltinRegistry().Supports(engine.ProviderID(id))
}

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
	if backend, ok := base.RendererImplementation(cfg.Backend); ok {
		return backend(synthesisPlan, cfg)
	}
	return nil, fmt.Errorf("unknown renderer backend %q", cfg.Backend)
}

// renderTestConcatは、WORLDのライブラリなしで合成の流れを確かめるテスト用の描画。原音を音程も長さも変えずに並べる。
func renderTestConcat(synthesisPlan *plan.Plan, cfg Config) (*audio.PCM, error) {
	if synthesisPlan == nil || len(synthesisPlan.Units) == 0 {
		return nil, errors.New("empty synthesis plan")
	}
	cache := base.NewSourceCache()
	sampleRate := 0
	var mix []float64
	for _, unit := range synthesisPlan.Units {
		if err := base.ContextError(cfg.Context); err != nil {
			return nil, err
		}
		if unit.Silent {
			continue
		}
		mono, err := cache.LoadMono(unit.Source)
		if err != nil {
			return nil, fmt.Errorf("read unit %q (%s): %w", unit.Alias, unit.Source, err)
		}
		if sampleRate == 0 {
			sampleRate = mono.SampleRate
		}
		if mono.SampleRate != sampleRate {
			if mono, err = cache.LoadNormalized(unit.Source, sampleRate); err != nil {
				return nil, fmt.Errorf("normalize unit %q: %w", unit.Alias, err)
			}
		}
		trimmed, err := audio.TrimPCM(mono, unit.OffsetMS, unit.CutoffMS)
		if err != nil {
			return nil, fmt.Errorf("trim unit %q: %w", unit.Alias, err)
		}
		wave := base.PcmFloats(trimmed.Data)
		start := max(0, base.MsToFramesSigned(unit.NoteStartMS-unit.PreutteranceMS, sampleRate))
		length := min(len(wave), base.MsToFrames(unit.PreutteranceMS+unit.DurationMS, sampleRate))
		if end := start + length; end > len(mix) {
			mix = append(mix, make([]float64, end-len(mix))...)
		}
		for frame := range length {
			mix[start+frame] += wave[frame]
		}
	}
	if sampleRate == 0 {
		return nil, errors.New("render produced no samples")
	}
	if minimum := base.MsToFrames(synthesisPlan.DurationMS, sampleRate); len(mix) < minimum {
		mix = append(mix, make([]float64, minimum-len(mix))...)
	}
	for frame, value := range mix {
		mix[frame] = math.Max(-1, math.Min(1, value))
	}
	return &audio.PCM{SampleRate: sampleRate, Channels: 1, Data: base.FloatPCM(mix)}, nil
}
