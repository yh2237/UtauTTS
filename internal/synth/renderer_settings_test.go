package synth

import (
	"encoding/json"
	"testing"

	"utautts/internal/render"
	"utautts/internal/tts"
)

// defaultOnlySpecsはtyped経路を外し、既定解決だけを観測できるようにしたコピー。
func defaultOnlySpecs() []rendererSettingSpec {
	specs := make([]rendererSettingSpec, len(rendererSettingSpecs))
	copy(specs, rendererSettingSpecs)
	for index := range specs {
		specs[index].typed = nil
	}
	return specs
}

// specテーブルの既定がConfig/ProviderOptions/Classic選択へ反映される。
func TestRendererSettingSpecDefaults(t *testing.T) {
	cfg := tts.Config{}
	options := render.ProviderOptions{}
	resolution := resolveRendererSettingsWith(defaultOnlySpecs(), Request{}, &cfg, &options)

	if cfg.MoraDurationMS != DefaultMoraDurationMS || cfg.PauseDurationMS != DefaultPauseDurationMS ||
		cfg.LeadingPreutteranceMS != 0 || cfg.IntonationStrength != DefaultIntonationStrength {
		t.Fatalf("timing defaults = %#v", cfg)
	}
	if cfg.ContextDuration == nil || *cfg.ContextDuration != DefaultContextDuration ||
		cfg.ContextDurationStrength != DefaultContextDurationStrength {
		t.Fatalf("context defaults = %#v", cfg)
	}
	if cfg.BoundaryTone == nil || *cfg.BoundaryTone != DefaultBoundaryTone ||
		cfg.BoundaryToneStrength != DefaultBoundaryToneStrength {
		t.Fatalf("boundary defaults = %#v", cfg)
	}
	if cfg.StretchAdapt == nil || *cfg.StretchAdapt != DefaultStretchAdapt ||
		cfg.StretchAdaptStrength != DefaultStretchAdaptStrength {
		t.Fatalf("stretch defaults = %#v", cfg)
	}
	if cfg.PauseContext == nil || *cfg.PauseContext != DefaultPauseContext ||
		cfg.PauseContextStrength != DefaultPauseContextStrength {
		t.Fatalf("pause defaults = %#v", cfg)
	}
	if cfg.EnglishWeakForm == nil || *cfg.EnglishWeakForm != DefaultEnglishWeakForm {
		t.Fatalf("weak form default = %#v", cfg)
	}
	if options.DiffSinger != (render.DiffSingerOptions{}) {
		t.Fatalf("diffsinger defaults = %#v", options.DiffSinger)
	}
	if resolution.Resampler != "" || resolution.Wavtool != "builtin" {
		t.Fatalf("classic tool defaults = %#v", resolution)
	}
	if len(options.Renderer) != 0 || len(options.RendererDiagnostics) != 0 {
		t.Fatalf("unexpected provider values: %#v", options)
	}
}

// typedフィールドがテーブル経由でConfig/ProviderOptions/Classic選択へ反映される。
func TestRendererSettingSpecTyped(t *testing.T) {
	cfg := tts.Config{}
	options := render.ProviderOptions{}
	resolution := resolveRendererSettingsWith(rendererSettingSpecs, Request{
		MoraDurationMS: 111, PauseDurationMS: 222, LeadingPreutteranceMS: 33, IntonationStrength: 1.5,
		ContextDuration: false, ContextDurationStrength: 0.4,
		BoundaryTone: false, BoundaryToneStrength: 0.6,
		StretchAdapt: false, StretchAdaptStrength: 0.7,
		PauseContext: false, PauseContextStrength: 0.8,
		EnglishWeakForm: false,
		DiffSingerSteps: 17, DiffSingerExpr: 0.9, DiffSingerDurationMix: 0.2, DiffSingerPitchMix: 0.3,
		Resampler: "nested/resampler.exe", Wavtool: "wavtool.exe",
	}, &cfg, &options)

	if cfg.MoraDurationMS != 111 || cfg.PauseDurationMS != 222 || cfg.LeadingPreutteranceMS != 33 || cfg.IntonationStrength != 1.5 {
		t.Fatalf("timing typed values = %#v", cfg)
	}
	if cfg.ContextDuration == nil || *cfg.ContextDuration || cfg.ContextDurationStrength != 0.4 {
		t.Fatalf("context typed values = %#v", cfg)
	}
	if cfg.BoundaryTone == nil || *cfg.BoundaryTone || cfg.BoundaryToneStrength != 0.6 {
		t.Fatalf("boundary typed values = %#v", cfg)
	}
	if cfg.StretchAdapt == nil || *cfg.StretchAdapt || cfg.StretchAdaptStrength != 0.7 {
		t.Fatalf("stretch typed values = %#v", cfg)
	}
	if cfg.PauseContext == nil || *cfg.PauseContext || cfg.PauseContextStrength != 0.8 {
		t.Fatalf("pause typed values = %#v", cfg)
	}
	if cfg.EnglishWeakForm == nil || *cfg.EnglishWeakForm {
		t.Fatalf("weak form typed value = %#v", cfg)
	}
	if options.DiffSinger.Steps != 17 || options.DiffSinger.Expr != 0.9 ||
		options.DiffSinger.DurationMix != 0.2 || options.DiffSinger.PitchMix != 0.3 {
		t.Fatalf("diffsinger typed values = %#v", options.DiffSinger)
	}
	if resolution.Resampler != "nested/resampler.exe" || resolution.Wavtool != "wavtool.exe" {
		t.Fatalf("classic typed values = %#v", resolution)
	}
}

// typedよりmapを優先し、未知idはProviderOptions.Rendererへ回す。
func TestRendererSettingSpecMapOverridesTyped(t *testing.T) {
	cfg := tts.Config{}
	options := render.ProviderOptions{}
	resolution := resolveRendererSettingsWith(rendererSettingSpecs, Request{
		MoraDurationMS: 111, ContextDuration: true, DiffSingerSteps: 17,
		Resampler: "typed-resampler.exe", Wavtool: "typed-wavtool.exe",
		RendererSettings: map[string]json.RawMessage{
			"mora_duration_ms": json.RawMessage(`123`),
			"context_duration": json.RawMessage(`false`),
			"diffsinger_steps": json.RawMessage(`5`),
			"resampler":        json.RawMessage(`"map-resampler.exe"`),
			"wavtool":          json.RawMessage(`"map-wavtool.exe"`),
			"custom_option":    json.RawMessage(`"value"`),
		},
	}, &cfg, &options)

	if cfg.MoraDurationMS != 123 || cfg.ContextDuration == nil || *cfg.ContextDuration {
		t.Fatalf("map did not override typed values: %#v", cfg)
	}
	if options.DiffSinger.Steps != 5 {
		t.Fatalf("map diffsinger steps = %#v", options.DiffSinger)
	}
	if resolution.Resampler != "map-resampler.exe" || resolution.Wavtool != "map-wavtool.exe" {
		t.Fatalf("map classic tools = %#v", resolution)
	}
	if options.Renderer["custom_option"] != "value" {
		t.Fatalf("unknown renderer setting = %#v", options.Renderer)
	}
}

// typed経由とmap経由は同じ解決結果になり、両方ある場合はmapが勝つ。
func TestRendererSettingSpecTypedAndMapAgree(t *testing.T) {
	typedRequest := Request{
		MoraDurationMS: 111, ContextDuration: false, ContextDurationStrength: 0.4,
		DiffSingerSteps: 17, Resampler: "resampler.exe",
	}
	typedCfg := tts.Config{}
	typedOptions := render.ProviderOptions{}
	typedResolution := resolveRendererSettingsWith(rendererSettingSpecs, typedRequest, &typedCfg, &typedOptions)

	mapCfg := tts.Config{}
	mapOptions := render.ProviderOptions{}
	mapResolution := resolveRendererSettingsWith(rendererSettingSpecs, Request{RendererSettings: map[string]json.RawMessage{
		"mora_duration_ms":          json.RawMessage(`111`),
		"context_duration":          json.RawMessage(`false`),
		"context_duration_strength": json.RawMessage(`0.4`),
		"diffsinger_steps":          json.RawMessage(`17`),
		"resampler":                 json.RawMessage(`"resampler.exe"`),
	}}, &mapCfg, &mapOptions)

	if typedCfg.MoraDurationMS != mapCfg.MoraDurationMS ||
		typedCfg.ContextDurationStrength != mapCfg.ContextDurationStrength ||
		*typedCfg.ContextDuration != *mapCfg.ContextDuration ||
		typedOptions.DiffSinger.Steps != mapOptions.DiffSinger.Steps ||
		typedResolution.Resampler != mapResolution.Resampler {
		t.Fatalf("typed and map disagree: typed=%#v/%#v map=%#v/%#v", typedCfg, typedOptions, mapCfg, mapOptions)
	}
}

// テーブルへ1行足すだけで新しい設定（typed無し）も既定とmap上書きが解決できる。
func TestRendererSettingSpecNewRowResolves(t *testing.T) {
	dummy := rendererSettingSpec{
		id: "test_new_setting", kind: rendererSettingKindNumber, defaultValue: 5.0,
		apply: func(value any, cfg *tts.Config, _ *render.ProviderOptions, _ *rendererSettingsResolution) {
			cfg.BoundaryBridgeMS = value.(float64)
		},
	}
	specs := append(append([]rendererSettingSpec(nil), rendererSettingSpecs...), dummy)

	defaultCfg := tts.Config{}
	defaultOptions := render.ProviderOptions{}
	resolveRendererSettingsWith(specs, Request{}, &defaultCfg, &defaultOptions)
	if defaultCfg.BoundaryBridgeMS != 5.0 {
		t.Fatalf("new setting default = %v", defaultCfg.BoundaryBridgeMS)
	}

	overrideCfg := tts.Config{}
	overrideOptions := render.ProviderOptions{}
	resolveRendererSettingsWith(specs, Request{RendererSettings: map[string]json.RawMessage{
		"test_new_setting": json.RawMessage(`9`),
	}}, &overrideCfg, &overrideOptions)
	if overrideCfg.BoundaryBridgeMS != 9.0 {
		t.Fatalf("new setting override = %v", overrideCfg.BoundaryBridgeMS)
	}
	if _, leaked := overrideOptions.Renderer["test_new_setting"]; leaked {
		t.Fatalf("known new setting leaked into provider values: %#v", overrideOptions.Renderer)
	}
}
