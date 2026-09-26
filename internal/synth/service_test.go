package synth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"utautts/internal/engine"
	"utautts/internal/plan"
	"utautts/internal/plugin"
	"utautts/internal/render"
	"utautts/internal/tts"
)

func boolPointer(value bool) *bool { return &value }

type testVoicebankResolver struct{ path string }

func testRenderer(id, provider string) plugin.Renderer {
	return plugin.Renderer{
		ManifestVersion: 2, Kind: "synthesis-engine", ID: id, DisplayName: id,
		Contract: "unit-renderer", Provider: provider, ProviderVersion: "1",
	}
}

func (resolver testVoicebankResolver) Resolve(string) (string, bool) {
	return resolver.path, true
}

func TestClassicUtauResolvesSelectedTools(t *testing.T) {
	directory := t.TempDir()
	resamplerPath := filepath.Join(directory, "resampler")
	wavtoolPath := filepath.Join(directory, "wavtool")
	for _, path := range []string{resamplerPath, wavtoolPath} {
		if err := os.WriteFile(path, []byte("tool"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	catalog := &plugin.Catalog{
		Renderers:  []plugin.Renderer{testRenderer("classic-utau", "utau-external-resampler")},
		Resamplers: []plugin.ClassicTool{{ID: "nested/resampler.exe", Path: resamplerPath}},
		Wavtools:   []plugin.ClassicTool{{ID: "builtin", BuiltIn: true}, {ID: "wavtool.exe", Path: wavtoolPath}},
	}
	service := NewService(catalog, "classic-utau", "", "", "", testVoicebankResolver{path: "voicebank"})
	cfg, renderer, options, err := service.config(Request{
		Renderer: "classic-utau", Resampler: "nested/resampler.exe", Wavtool: "wavtool.exe",
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if renderer != "classic-utau" || options.Classic.ResamplerPath != resamplerPath || options.Classic.WavtoolPath != wavtoolPath {
		t.Fatalf("classic UTAU config = %#v, provider options %#v, renderer %q", cfg, options, renderer)
	}
}

func TestResolveSynthesisUsesDirectVoicebankPathAndNormalizesKana(t *testing.T) {
	service := NewService(&plugin.Catalog{
		Renderers: []plugin.Renderer{testRenderer("waveform", "waveform")},
	}, "waveform", "", "", "", nil)
	resolved, err := service.ResolveSynthesis(Request{
		VoicebankPath: "voicebank", Kana: "あ", Renderer: "waveform",
		ReleaseMS: 20, ReleaseSet: true, BoundaryBridgeMS: 12,
		CVVCTiming: "sequential", JoinModelPath: "join.json",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Config.VoicebankPath != "voicebank" || resolved.Config.Reading != "あ" {
		t.Fatalf("resolved config = %#v", resolved.Config)
	}
	if !resolved.Config.ReleaseSet || resolved.Config.ReleaseMS != 20 ||
		resolved.Config.BoundaryBridgeMS != 12 || resolved.Config.CVVCTiming != "sequential" ||
		resolved.Config.JoinModelPath != "join.json" {
		t.Fatalf("CLI settings were not preserved: %#v", resolved.Config)
	}
}

// 同梱manifestのtiming既定はGoのcanonical値と一致させる。
func TestBundledRendererManifestsUseCanonicalTimingDefaults(t *testing.T) {
	catalog, err := plugin.DiscoverWithDefaults(nil, nil, func(string) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	// DiffSinger manifestはWindows限定のため、非Windowsでは3件になる。
	if len(catalog.Renderers) < 3 {
		t.Fatalf("renderers = %d, want at least 3", len(catalog.Renderers))
	}
	want := map[string]float64{
		"mora_duration_ms":  plan.DefaultMoraDurationMS,
		"pause_duration_ms": plan.DefaultPauseDurationMS,
	}
	for _, renderer := range catalog.Renderers {
		for _, setting := range renderer.Settings {
			expected, ok := want[setting.ID]
			if !ok {
				continue
			}
			if got, ok := setting.Default.(float64); !ok || got != expected {
				t.Errorf("%s %s default = %v, want %v", renderer.ID, setting.ID, setting.Default, expected)
			}
		}
	}
}

// renderer_settingsの既知idが型ごとにConfig/ProviderOptionsへ反映される。
func TestApplyRendererSettingsKnownIDs(t *testing.T) {
	cfg := tts.Config{MoraDurationMS: 140, ContextDuration: boolPointer(true)}
	options := render.ProviderOptions{}
	settings := map[string]json.RawMessage{
		"mora_duration_ms":          json.RawMessage(`130`),
		"pause_duration_ms":         json.RawMessage(`190`),
		"leading_preutterance_ms":   json.RawMessage(`45`),
		"intonation_strength":       json.RawMessage(`2.5`),
		"context_duration":          json.RawMessage(`false`),
		"context_duration_strength": json.RawMessage(`0.5`),
		"boundary_tone":             json.RawMessage(`false`),
		"boundary_tone_strength":    json.RawMessage(`0.5`),
		"stretch_adapt":             json.RawMessage(`false`),
		"stretch_adapt_strength":    json.RawMessage(`0.5`),
		"pause_context":             json.RawMessage(`false`),
		"pause_context_strength":    json.RawMessage(`0.5`),
		"english_weak_form":         json.RawMessage(`false`),
		"diffsinger_steps":          json.RawMessage(`42`),
		"diffsinger_expr":           json.RawMessage(`0.75`),
		"diffsinger_duration_mix":   json.RawMessage(`0.25`),
		"diffsinger_pitch_mix":      json.RawMessage(`0.35`),
	}
	resolution := applyRendererSettings(settings, &cfg, &options)
	if len(options.RendererDiagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", options.RendererDiagnostics)
	}
	if resolution.Resampler != "" || resolution.Wavtool != "" {
		t.Fatalf("classic tools were set: %#v", resolution)
	}
	if cfg.MoraDurationMS != 130 || cfg.PauseDurationMS != 190 || cfg.LeadingPreutteranceMS != 45 || cfg.IntonationStrength != 2.5 {
		t.Fatalf("timing settings = %#v", cfg)
	}
	if cfg.ContextDuration == nil || *cfg.ContextDuration || cfg.ContextDurationStrength != 0.5 {
		t.Fatalf("context duration settings = %#v", cfg)
	}
	if cfg.BoundaryTone == nil || *cfg.BoundaryTone || cfg.BoundaryToneStrength != 0.5 {
		t.Fatalf("boundary tone settings = %#v", cfg)
	}
	if cfg.StretchAdapt == nil || *cfg.StretchAdapt || cfg.StretchAdaptStrength != 0.5 {
		t.Fatalf("stretch adapt settings = %#v", cfg)
	}
	if cfg.PauseContext == nil || *cfg.PauseContext || cfg.PauseContextStrength != 0.5 {
		t.Fatalf("pause context settings = %#v", cfg)
	}
	if cfg.EnglishWeakForm == nil || *cfg.EnglishWeakForm {
		t.Fatalf("english weak form settings = %#v", cfg)
	}
	if options.DiffSinger.Steps != 42 || options.DiffSinger.Expr != 0.75 ||
		options.DiffSinger.DurationMix != 0.25 || options.DiffSinger.PitchMix != 0.35 {
		t.Fatalf("diffsinger settings = %#v", options.DiffSinger)
	}
}

// 未知idはエラーにせずProviderOptions.Rendererへ保持する。
func TestApplyRendererSettingsUnknownGoesToProviderOptions(t *testing.T) {
	cfg := tts.Config{}
	options := render.ProviderOptions{}
	applyRendererSettings(map[string]json.RawMessage{
		"world_mode":  json.RawMessage(`"adaptive"`),
		"world_gain":  json.RawMessage(`0.8`),
		"world_ready": json.RawMessage(`true`),
	}, &cfg, &options)
	if options.Renderer["world_mode"] != "adaptive" || options.Renderer["world_gain"] != 0.8 || options.Renderer["world_ready"] != true {
		t.Fatalf("provider values = %#v", options.Renderer)
	}
	if len(options.RendererDiagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", options.RendererDiagnostics)
	}
}

// 型不一致はエラーではなく無視し、診断だけ残す。
func TestApplyRendererSettingsTypeMismatchIsIgnored(t *testing.T) {
	cfg := tts.Config{MoraDurationMS: 140, ContextDuration: boolPointer(true)}
	options := render.ProviderOptions{}
	applyRendererSettings(map[string]json.RawMessage{
		"mora_duration_ms": json.RawMessage(`true`),
		"context_duration": json.RawMessage(`"yes"`),
		"resampler":        json.RawMessage(`123`),
	}, &cfg, &options)
	if cfg.MoraDurationMS != 140 || cfg.ContextDuration == nil || !*cfg.ContextDuration {
		t.Fatalf("mismatched values changed config: %#v", cfg)
	}
	if len(options.RendererDiagnostics) != 3 {
		t.Fatalf("diagnostics = %v", options.RendererDiagnostics)
	}
}

// mapが与えられたら固定フィールドより優先し、無ければ固定フィールドを保つ。
func TestConfigRendererSettingsOverrideFixedFields(t *testing.T) {
	service := NewService(&plugin.Catalog{
		Renderers: []plugin.Renderer{testRenderer("waveform", "waveform")},
	}, "waveform", "", "", "", nil)
	cfg, _, options, err := service.config(Request{
		Renderer: "waveform", ContextDuration: true, MoraDurationMS: 140,
		RendererSettings: map[string]json.RawMessage{
			"context_duration": json.RawMessage(`false`),
			"mora_duration_ms": json.RawMessage(`123`),
			"custom_option":    json.RawMessage(`"value"`),
		},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ContextDuration == nil || *cfg.ContextDuration || cfg.MoraDurationMS != 123 {
		t.Fatalf("renderer settings did not override fixed fields: %#v", cfg)
	}
	if options.Renderer["custom_option"] != "value" {
		t.Fatalf("unknown renderer setting = %#v", options.Renderer)
	}

	cfg, _, _, err = service.config(Request{Renderer: "waveform", ContextDuration: true, MoraDurationMS: 140}, false)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ContextDuration == nil || !*cfg.ContextDuration || cfg.MoraDurationMS != 140 {
		t.Fatalf("fixed fields without a map changed: %#v", cfg)
	}
}

// renderer_settings経由でもClassicツール選択が効く。
func TestRendererSettingsRouteClassicTools(t *testing.T) {
	directory := t.TempDir()
	resamplerPath := filepath.Join(directory, "resampler")
	wavtoolPath := filepath.Join(directory, "wavtool")
	for _, path := range []string{resamplerPath, wavtoolPath} {
		if err := os.WriteFile(path, []byte("tool"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	catalog := &plugin.Catalog{
		Renderers:  []plugin.Renderer{testRenderer("classic-utau", "utau-external-resampler")},
		Resamplers: []plugin.ClassicTool{{ID: "nested/resampler.exe", Path: resamplerPath}},
		Wavtools:   []plugin.ClassicTool{{ID: "builtin", BuiltIn: true}, {ID: "wavtool.exe", Path: wavtoolPath}},
	}
	service := NewService(catalog, "classic-utau", "", "", "", testVoicebankResolver{path: "voicebank"})
	_, _, options, err := service.config(Request{
		Renderer: "classic-utau",
		RendererSettings: map[string]json.RawMessage{
			"resampler": json.RawMessage(`"nested/resampler.exe"`),
			"wavtool":   json.RawMessage(`"wavtool.exe"`),
		},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if options.Classic.ResamplerPath != resamplerPath || options.Classic.WavtoolPath != wavtoolPath {
		t.Fatalf("classic tools from renderer_settings = %#v", options.Classic)
	}
}

func TestClassicUtauRejectsUnknownResampler(t *testing.T) {
	catalog := &plugin.Catalog{
		Renderers: []plugin.Renderer{testRenderer("classic-utau", "utau-external-resampler")},
		Wavtools:  []plugin.ClassicTool{{ID: "builtin", BuiltIn: true}},
	}
	service := NewService(catalog, "classic-utau", "", "", "", testVoicebankResolver{path: "voicebank"})
	if _, _, _, err := service.config(Request{Renderer: "classic-utau", Resampler: "missing", Wavtool: "builtin"}, true); err == nil {
		t.Fatal("unknown resampler was accepted")
	}
}

func TestClassicUtauRejectsMissingToolExecutable(t *testing.T) {
	catalog := &plugin.Catalog{
		Renderers:  []plugin.Renderer{testRenderer("classic-utau", "utau-external-resampler")},
		Resamplers: []plugin.ClassicTool{{ID: "missing.exe", Path: filepath.Join(t.TempDir(), "missing.exe")}},
		Wavtools:   []plugin.ClassicTool{{ID: "builtin", BuiltIn: true}},
	}
	service := NewService(catalog, "classic-utau", "", "", "", testVoicebankResolver{path: "voicebank"})
	if _, err := service.ResolveClassicTools("missing.exe", "builtin"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing tool error = %v", err)
	}
}

func TestResolveRendererSeparatesPublicIDFromProvider(t *testing.T) {
	service := NewService(&plugin.Catalog{Renderers: []plugin.Renderer{
		func() plugin.Renderer {
			renderer := testRenderer("friendly-world", "utautts-world-phrase")
			renderer.DisplayName = "Friendly WORLD"
			return renderer
		}(),
	}}, "", "", "", "", nil)

	resolved, err := service.ResolveRenderer("friendly-world")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.PublicID() != "friendly-world" {
		t.Fatalf("public id = %q", resolved.PublicID())
	}
	if resolved.Provider.ID != "utautts-world-phrase" || resolved.Definition.Contract != engine.ContractUnitRenderer {
		t.Fatalf("provider resolution = %#v", resolved)
	}
}

func TestResolveRendererDoesNotFallbackForMissingExplicitID(t *testing.T) {
	service := NewService(&plugin.Catalog{Renderers: []plugin.Renderer{testRenderer("default", "waveform")}}, "default", "", "", "", nil)
	if _, err := service.ResolveRenderer("removed"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing explicit renderer error = %v", err)
	}
}

func TestRendererAvailabilityReportsMissingRuntimeWithoutRejectingListing(t *testing.T) {
	service := NewService(&plugin.Catalog{Renderers: []plugin.Renderer{
		func() plugin.Renderer {
			renderer := testRenderer("world", "utautts-world-phrase")
			renderer.DisplayName = "WORLD"
			return renderer
		}(),
	}}, "", "", "", "", nil)
	availability := service.RendererAvailability()["world"]
	if availability.Available || len(availability.Issues) == 0 {
		t.Fatalf("availability = %#v", availability)
	}
}

func TestSynthesisConfigRejectsUnavailableRendererRuntime(t *testing.T) {
	service := NewService(&plugin.Catalog{Renderers: []plugin.Renderer{
		func() plugin.Renderer {
			renderer := testRenderer("world", "utautts-world-phrase")
			renderer.DisplayName = "WORLD"
			return renderer
		}(),
	}}, "", "", "", "", testVoicebankResolver{path: "voicebank"})
	if _, _, _, err := service.config(Request{Renderer: "world"}, true); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unavailable runtime error = %v", err)
	}
}

func TestConfigUsesExplicitModelPath(t *testing.T) {
	service := NewService(&plugin.Catalog{
		Renderers: []plugin.Renderer{testRenderer("waveform", "waveform")},
	}, "waveform", "", "", "", nil)
	cfg, renderer, _, err := service.config(Request{
		ModelID: "not-in-catalog", ModelPath: "out/working-model.json", Renderer: "waveform",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if renderer != "waveform" || cfg.ProsodyModelPath != "out/working-model.json" {
		t.Fatalf("explicit model path was not preserved: renderer=%q config=%#v", renderer, cfg)
	}
}
