package synth

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/yh2237/UtauTTS/internal/render"
	"github.com/yh2237/UtauTTS/internal/settings"
	"github.com/yh2237/UtauTTS/internal/tts"
)

// 数値はnumber/integerともfloat64で扱う。
type rendererSettingKind int

const (
	rendererSettingKindNumber rendererSettingKind = iota
	rendererSettingKindBoolean
)

// 既定値はinternal/settingsから取得し、このテーブルで適用先を指定する。
type rendererSettingSpec struct {
	id           string
	kind         rendererSettingKind
	defaultValue any
	apply        func(value any, cfg *tts.Config, options *render.ProviderOptions)
	// nilならrenderer_settingsだけを受け取る。
	typed func(request Request) any
}

// rendererSettingSpecsはGoが既知の設定を定義する。manifestにない項目もAPI互換用に残す。
var rendererSettingSpecs = []rendererSettingSpec{
	numberSetting("mora_duration_ms",
		func(r Request) any { return r.MoraDurationMS },
		func(value float64, cfg *tts.Config, _ *render.ProviderOptions) { cfg.MoraDurationMS = value }),
	numberSetting("pause_duration_ms",
		func(r Request) any { return r.PauseDurationMS },
		func(value float64, cfg *tts.Config, _ *render.ProviderOptions) { cfg.PauseDurationMS = value }),
	numberSetting("leading_preutterance_ms",
		func(r Request) any { return r.LeadingPreutteranceMS },
		func(value float64, cfg *tts.Config, _ *render.ProviderOptions) { cfg.LeadingPreutteranceMS = value }),
	numberSetting("intonation_strength",
		func(r Request) any { return r.IntonationStrength },
		func(value float64, cfg *tts.Config, _ *render.ProviderOptions) { cfg.IntonationStrength = value }),
	boolSetting("context_duration",
		func(r Request) any { return r.ContextDuration },
		func(value bool, cfg *tts.Config) { cfg.ContextDuration = &value }),
	numberSetting("context_duration_strength",
		func(r Request) any { return r.ContextDurationStrength },
		func(value float64, cfg *tts.Config, _ *render.ProviderOptions) { cfg.ContextDurationStrength = value }),
	boolSetting("boundary_tone",
		func(r Request) any { return r.BoundaryTone },
		func(value bool, cfg *tts.Config) { cfg.BoundaryTone = &value }),
	numberSetting("boundary_tone_strength",
		func(r Request) any { return r.BoundaryToneStrength },
		func(value float64, cfg *tts.Config, _ *render.ProviderOptions) { cfg.BoundaryToneStrength = value }),
	boolSetting("stretch_adapt",
		func(r Request) any { return r.StretchAdapt },
		func(value bool, cfg *tts.Config) { cfg.StretchAdapt = &value }),
	numberSetting("stretch_adapt_strength",
		func(r Request) any { return r.StretchAdaptStrength },
		func(value float64, cfg *tts.Config, _ *render.ProviderOptions) { cfg.StretchAdaptStrength = value }),
	boolSetting("pause_context",
		func(r Request) any { return r.PauseContext },
		func(value bool, cfg *tts.Config) { cfg.PauseContext = &value }),
	numberSetting("pause_context_strength",
		func(r Request) any { return r.PauseContextStrength },
		func(value float64, cfg *tts.Config, _ *render.ProviderOptions) { cfg.PauseContextStrength = value }),
	{
		id: "timing_warp", kind: rendererSettingKindBoolean, defaultValue: settings.Bool("timing_warp"),
		typed: func(r Request) any { return r.Worldline.TimingWarpEnabled() },
		apply: func(value any, _ *tts.Config, options *render.ProviderOptions) {
			enabled := value.(bool)
			options.Worldline.TimingWarp = &enabled
		},
	},
	{
		id: "microprosody", kind: rendererSettingKindBoolean, defaultValue: settings.Bool("microprosody"),
		typed: func(r Request) any { return r.Worldline.MicroprosodyEnabled() },
		apply: func(value any, _ *tts.Config, options *render.ProviderOptions) {
			enabled := value.(bool)
			options.Worldline.Microprosody = &enabled
		},
	},
}

func numberSetting(id string, typed func(Request) any, apply func(float64, *tts.Config, *render.ProviderOptions)) rendererSettingSpec {
	return rendererSettingSpec{
		id: id, kind: rendererSettingKindNumber, defaultValue: settings.Number(id), typed: typed,
		apply: func(value any, cfg *tts.Config, options *render.ProviderOptions) {
			apply(value.(float64), cfg, options)
		},
	}
}

func boolSetting(id string, typed func(Request) any, apply func(bool, *tts.Config)) rendererSettingSpec {
	return rendererSettingSpec{
		id: id, kind: rendererSettingKindBoolean, defaultValue: settings.Bool(id), typed: typed,
		apply: func(value any, cfg *tts.Config, _ *render.ProviderOptions) {
			apply(value.(bool), cfg)
		},
	}
}

// 優先順はrenderer_settings、型付き設定、既定値。
func resolveRendererSettings(request Request, cfg *tts.Config, options *render.ProviderOptions) {
	for _, spec := range rendererSettingSpecs {
		value := spec.defaultValue
		if spec.typed != nil {
			value = spec.typed(request)
		}
		spec.apply(value, cfg, options)
	}
	applyRendererSettingsMap(rendererSettingSpecs, request.RendererSettings, cfg, options)
}

// 既知IDは設定へ、未知IDは実装固有値へ渡す。型不一致と範囲外の値は診断を残して無視する。
func applyRendererSettingsMap(specs []rendererSettingSpec, settings map[string]json.RawMessage, cfg *tts.Config, options *render.ProviderOptions) {
	if len(settings) == 0 {
		return
	}
	unknown := make(map[string]any, len(settings))
	for id, raw := range settings {
		spec, known := rendererSettingSpecFor(specs, id)
		if !known {
			var value any
			if err := json.Unmarshal(raw, &value); err != nil {
				options.RendererDiagnostics = append(options.RendererDiagnostics,
					fmt.Sprintf("renderer setting %q was ignored: %v", id, err))
				continue
			}
			unknown[id] = value
			continue
		}
		value, ok := spec.decode(id, raw, options)
		if !ok {
			continue
		}
		spec.apply(value, cfg, options)
	}
	if len(unknown) > 0 {
		options.Renderer = unknown
	}
}

func rendererSettingSpecFor(specs []rendererSettingSpec, id string) (rendererSettingSpec, bool) {
	for _, spec := range specs {
		if spec.id == id {
			return spec, true
		}
	}
	return rendererSettingSpec{}, false
}

func (spec rendererSettingSpec) decode(id string, raw json.RawMessage, options *render.ProviderOptions) (any, bool) {
	switch spec.kind {
	case rendererSettingKindBoolean:
		return rendererSettingBool(id, raw, options)
	default:
		return rendererSettingNumber(id, raw, options)
	}
}

func rendererSettingNumber(id string, raw json.RawMessage, options *render.ProviderOptions) (float64, bool) {
	var value float64
	if err := json.Unmarshal(raw, &value); err != nil {
		options.RendererDiagnostics = append(options.RendererDiagnostics,
			fmt.Sprintf("renderer setting %q expects a number: %v", id, err))
		return 0, false
	}
	if setting, known := settings.Lookup(id); known &&
		(math.IsNaN(value) || setting.Min != nil && value < *setting.Min || setting.Max != nil && value > *setting.Max) {
		options.RendererDiagnostics = append(options.RendererDiagnostics,
			fmt.Sprintf("renderer setting %q is outside the supported range: %v", id, value))
		return 0, false
	}
	return value, true
}

func rendererSettingBool(id string, raw json.RawMessage, options *render.ProviderOptions) (bool, bool) {
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		options.RendererDiagnostics = append(options.RendererDiagnostics,
			fmt.Sprintf("renderer setting %q expects a boolean: %v", id, err))
		return false, false
	}
	return value, true
}
