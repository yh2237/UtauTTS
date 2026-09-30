package synth

import (
	"encoding/json"
	"fmt"

	"utautts/internal/render"
	"utautts/internal/tts"
)

type rendererSettingsResolution struct {
	Resampler string
	Wavtool   string
}

// rendererSettingKindはmanifestのtypeに対応する値の種類。数値はnumber/integerを区別せずfloat64で扱う。
type rendererSettingKind int

const (
	rendererSettingKindNumber rendererSettingKind = iota
	rendererSettingKindInteger
	rendererSettingKindBoolean
	rendererSettingKindString
)

// 新しい設定はmanifestとこのテーブルへ追加する。Requestの固定フィールドは増やさない。
type rendererSettingSpec struct {
	id   string
	kind rendererSettingKind
	// defaultValueはcanonicalな既定値。typedフィールドやrenderer_settingsが無い設定の土台になる。
	defaultValue any
	apply        func(value any, cfg *tts.Config, options *render.ProviderOptions, resolution *rendererSettingsResolution)
	// typedは互換用のtypedフィールドから値を取り出す。nilならmap経路のみ（新設定向け）。
	typed func(request Request) any
}

// rendererSettingSpecsはGoが既知の設定を定義する。manifestにない項目もAPI互換用に残す。
var rendererSettingSpecs = []rendererSettingSpec{
	numberSetting("mora_duration_ms", DefaultMoraDurationMS,
		func(r Request) any { return r.MoraDurationMS },
		func(value float64, cfg *tts.Config, _ *render.ProviderOptions) { cfg.MoraDurationMS = value }),
	numberSetting("pause_duration_ms", DefaultPauseDurationMS,
		func(r Request) any { return r.PauseDurationMS },
		func(value float64, cfg *tts.Config, _ *render.ProviderOptions) { cfg.PauseDurationMS = value }),
	numberSetting("leading_preutterance_ms", 0,
		func(r Request) any { return r.LeadingPreutteranceMS },
		func(value float64, cfg *tts.Config, _ *render.ProviderOptions) { cfg.LeadingPreutteranceMS = value }),
	numberSetting("intonation_strength", DefaultIntonationStrength,
		func(r Request) any { return r.IntonationStrength },
		func(value float64, cfg *tts.Config, _ *render.ProviderOptions) { cfg.IntonationStrength = value }),
	boolSetting("context_duration", DefaultContextDuration,
		func(r Request) any { return r.ContextDuration },
		func(value bool, cfg *tts.Config) { cfg.ContextDuration = &value }),
	numberSetting("context_duration_strength", DefaultContextDurationStrength,
		func(r Request) any { return r.ContextDurationStrength },
		func(value float64, cfg *tts.Config, _ *render.ProviderOptions) { cfg.ContextDurationStrength = value }),
	boolSetting("boundary_tone", DefaultBoundaryTone,
		func(r Request) any { return r.BoundaryTone },
		func(value bool, cfg *tts.Config) { cfg.BoundaryTone = &value }),
	numberSetting("boundary_tone_strength", DefaultBoundaryToneStrength,
		func(r Request) any { return r.BoundaryToneStrength },
		func(value float64, cfg *tts.Config, _ *render.ProviderOptions) { cfg.BoundaryToneStrength = value }),
	boolSetting("stretch_adapt", DefaultStretchAdapt,
		func(r Request) any { return r.StretchAdapt },
		func(value bool, cfg *tts.Config) { cfg.StretchAdapt = &value }),
	numberSetting("stretch_adapt_strength", DefaultStretchAdaptStrength,
		func(r Request) any { return r.StretchAdaptStrength },
		func(value float64, cfg *tts.Config, _ *render.ProviderOptions) { cfg.StretchAdaptStrength = value }),
	boolSetting("pause_context", DefaultPauseContext,
		func(r Request) any { return r.PauseContext },
		func(value bool, cfg *tts.Config) { cfg.PauseContext = &value }),
	numberSetting("pause_context_strength", DefaultPauseContextStrength,
		func(r Request) any { return r.PauseContextStrength },
		func(value float64, cfg *tts.Config, _ *render.ProviderOptions) { cfg.PauseContextStrength = value }),
	boolSetting("english_weak_form", DefaultEnglishWeakForm,
		func(r Request) any { return r.EnglishWeakForm },
		func(value bool, cfg *tts.Config) { cfg.EnglishWeakForm = &value }),
	integerSetting("diffsinger_steps", 0,
		func(r Request) any { return float64(r.DiffSingerSteps) },
		func(value float64, options *render.ProviderOptions) { options.DiffSinger.Steps = int64(value) }),
	numberSetting("diffsinger_expr", 0,
		func(r Request) any { return r.DiffSingerExpr },
		func(value float64, _ *tts.Config, options *render.ProviderOptions) { options.DiffSinger.Expr = value }),
	numberSetting("diffsinger_duration_mix", 0,
		func(r Request) any { return r.DiffSingerDurationMix },
		func(value float64, _ *tts.Config, options *render.ProviderOptions) {
			options.DiffSinger.DurationMix = value
		}),
	numberSetting("diffsinger_pitch_mix", 0,
		func(r Request) any { return r.DiffSingerPitchMix },
		func(value float64, _ *tts.Config, options *render.ProviderOptions) {
			options.DiffSinger.PitchMix = value
		}),
	stringSetting("resampler", "",
		func(r Request) any { return r.Resampler },
		func(value string, resolution *rendererSettingsResolution) { resolution.Resampler = value }),
	stringSetting("wavtool", "builtin",
		func(r Request) any { return r.Wavtool },
		func(value string, resolution *rendererSettingsResolution) { resolution.Wavtool = value }),
}

func numberSetting(id string, defaultValue float64, typed func(Request) any, apply func(float64, *tts.Config, *render.ProviderOptions)) rendererSettingSpec {
	return rendererSettingSpec{
		id: id, kind: rendererSettingKindNumber, defaultValue: defaultValue, typed: typed,
		apply: func(value any, cfg *tts.Config, options *render.ProviderOptions, _ *rendererSettingsResolution) {
			apply(value.(float64), cfg, options)
		},
	}
}

func integerSetting(id string, defaultValue float64, typed func(Request) any, apply func(float64, *render.ProviderOptions)) rendererSettingSpec {
	return rendererSettingSpec{
		id: id, kind: rendererSettingKindInteger, defaultValue: defaultValue, typed: typed,
		apply: func(value any, _ *tts.Config, options *render.ProviderOptions, _ *rendererSettingsResolution) {
			apply(value.(float64), options)
		},
	}
}

func boolSetting(id string, defaultValue bool, typed func(Request) any, apply func(bool, *tts.Config)) rendererSettingSpec {
	return rendererSettingSpec{
		id: id, kind: rendererSettingKindBoolean, defaultValue: defaultValue, typed: typed,
		apply: func(value any, cfg *tts.Config, _ *render.ProviderOptions, _ *rendererSettingsResolution) {
			apply(value.(bool), cfg)
		},
	}
}

func stringSetting(id string, defaultValue string, typed func(Request) any, apply func(string, *rendererSettingsResolution)) rendererSettingSpec {
	return rendererSettingSpec{
		id: id, kind: rendererSettingKindString, defaultValue: defaultValue, typed: typed,
		apply: func(value any, _ *tts.Config, _ *render.ProviderOptions, resolution *rendererSettingsResolution) {
			apply(value.(string), resolution)
		},
	}
}

// resolveRendererSettingsはrenderer設定を「既定→typed→renderer_settings」の順に解決する。
// typedは既存経路の互換用で、同じidをmapが持てばmapが優先される。
func resolveRendererSettings(request Request, cfg *tts.Config, options *render.ProviderOptions) rendererSettingsResolution {
	return resolveRendererSettingsWith(rendererSettingSpecs, request, cfg, options)
}

func resolveRendererSettingsWith(specs []rendererSettingSpec, request Request, cfg *tts.Config, options *render.ProviderOptions) rendererSettingsResolution {
	resolution := rendererSettingsResolution{}
	for _, spec := range specs {
		spec.apply(spec.defaultValue, cfg, options, &resolution)
	}
	for _, spec := range specs {
		if spec.typed == nil {
			continue
		}
		spec.apply(spec.typed(request), cfg, options, &resolution)
	}
	applyRendererSettingsMap(specs, request.RendererSettings, cfg, options, &resolution)
	return resolution
}

// applyRendererSettingsはmapだけを解決する。既存の呼び出し互換のためシグネチャを保つ。
func applyRendererSettings(settings map[string]json.RawMessage, cfg *tts.Config, options *render.ProviderOptions) rendererSettingsResolution {
	resolution := rendererSettingsResolution{}
	applyRendererSettingsMap(rendererSettingSpecs, settings, cfg, options, &resolution)
	return resolution
}

// 既知IDは設定へ、未知IDは実装固有値へ渡す。型不一致は診断を残して無視する。
func applyRendererSettingsMap(specs []rendererSettingSpec, settings map[string]json.RawMessage, cfg *tts.Config, options *render.ProviderOptions, resolution *rendererSettingsResolution) {
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
		spec.apply(value, cfg, options, resolution)
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
	case rendererSettingKindString:
		return rendererSettingString(id, raw, options)
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

func rendererSettingString(id string, raw json.RawMessage, options *render.ProviderOptions) (string, bool) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		options.RendererDiagnostics = append(options.RendererDiagnostics,
			fmt.Sprintf("renderer setting %q expects a string: %v", id, err))
		return "", false
	}
	return value, true
}
