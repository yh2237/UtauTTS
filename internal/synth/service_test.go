package synth

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/yh2237/UtauTTS/internal/engine"
	"github.com/yh2237/UtauTTS/internal/plugin"
	"github.com/yh2237/UtauTTS/internal/render"
	"github.com/yh2237/UtauTTS/internal/tts"
)

func boolPointer(value bool) *bool { return &value }

type testVoicebankResolver struct{ path string }

func testRenderer(id, provider string) plugin.Renderer {
	return plugin.Renderer{
		ManifestVersion: 2, Kind: "synthesis-engine", ID: id, DisplayName: id,
		Contract: "unit-renderer", Provider: provider, ProviderVersion: "1",
	}
}

func TestDefaultRequestResolvesSynthesisSettings(t *testing.T) {
	service := NewService(&plugin.Catalog{
		Renderers: []plugin.Renderer{testRenderer("test-concat", "test-concat")},
	}, "test-concat", "", "", testVoicebankResolver{path: "voicebank"})
	request := DefaultRequest()
	request.Method = tts.MethodWaveformSynthesis
	request.Language = "ja"
	request.VoicebankID = "bank"
	resolved, err := service.ResolveSynthesis(request)
	if err != nil {
		t.Fatal(err)
	}
	cfg, options := resolved.Config, resolved.ProviderOptions
	if cfg.MoraDurationMS != 120 || cfg.PauseDurationMS != 180 || cfg.IntonationStrength != 4 || !cfg.ApplyPitch {
		t.Fatalf("default timing/pitch = %#v", cfg)
	}
	if cfg.ContextDuration == nil || *cfg.ContextDuration {
		t.Fatalf("default context duration = %v", cfg.ContextDuration)
	}
	for name, enabled := range map[string]*bool{
		"boundary_tone": cfg.BoundaryTone, "stretch_adapt": cfg.StretchAdapt,
		"pause_context": cfg.PauseContext,
	} {
		if enabled == nil || !*enabled {
			t.Errorf("default %s = %v, want enabled", name, enabled)
		}
	}
	if !options.Worldline.TimingWarpEnabled() || !options.Worldline.MicroprosodyEnabled() || len(options.RendererDiagnostics) != 0 {
		t.Fatalf("default provider options = %#v", options)
	}
}

func (resolver testVoicebankResolver) Resolve(string) (string, bool) {
	return resolver.path, true
}

func TestResolveSynthesisUsesDirectVoicebankPathAndNormalizesKana(t *testing.T) {
	service := NewService(&plugin.Catalog{
		Renderers: []plugin.Renderer{testRenderer("test-concat", "test-concat")},
	}, "test-concat", "", "", nil)
	resolved, err := service.ResolveSynthesis(Request{
		VoicebankPath: "voicebank", Kana: "あ", Renderer: "test-concat", Method: tts.MethodWaveformSynthesis, Language: "ja",
		ReleaseMS: 20, ReleaseSet: true,
		CVVCTiming: "sequential",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Config.VoicebankPath != "voicebank" || resolved.Config.Reading != "あ" {
		t.Fatalf("resolved config = %#v", resolved.Config)
	}
	if !resolved.Config.ReleaseSet || resolved.Config.ReleaseMS != 20 ||
		resolved.Config.CVVCTiming != "sequential" {
		t.Fatalf("CLI settings were not preserved: %#v", resolved.Config)
	}
}

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
		"diffsinger_steps":          json.RawMessage(`42`),
		"diffsinger_expr":           json.RawMessage(`0.75`),
		"diffsinger_duration_mix":   json.RawMessage(`0.25`),
		"diffsinger_pitch_mix":      json.RawMessage(`0.35`),
	}
	applyRendererSettingsMap(rendererSettingSpecs, settings, &cfg, &options)
	if len(options.RendererDiagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", options.RendererDiagnostics)
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
	if options.DiffSinger.Steps != 42 || options.DiffSinger.Expr != 0.75 ||
		options.DiffSinger.DurationMix != 0.25 || options.DiffSinger.PitchMix != 0.35 {
		t.Fatalf("diffsinger settings = %#v", options.DiffSinger)
	}
}

func TestApplyRendererSettingsTypeMismatchIsIgnored(t *testing.T) {
	cfg := tts.Config{MoraDurationMS: 140, ContextDuration: boolPointer(true)}
	options := render.ProviderOptions{}
	applyRendererSettingsMap(rendererSettingSpecs, map[string]json.RawMessage{
		"mora_duration_ms":    json.RawMessage(`true`),
		"context_duration":    json.RawMessage(`"yes"`),
		"intonation_strength": json.RawMessage(`"high"`),
	}, &cfg, &options)
	if cfg.MoraDurationMS != 140 || cfg.ContextDuration == nil || !*cfg.ContextDuration {
		t.Fatalf("mismatched values changed config: %#v", cfg)
	}
	if len(options.RendererDiagnostics) != 3 {
		t.Fatalf("diagnostics = %v", options.RendererDiagnostics)
	}
}

func TestConfigRendererSettingsOverrideFixedFields(t *testing.T) {
	service := NewService(&plugin.Catalog{
		Renderers: []plugin.Renderer{testRenderer("test-concat", "test-concat")},
	}, "test-concat", "", "", nil)
	cfg, _, options, err := service.config(Request{
		Renderer: "test-concat", ContextDuration: true, MoraDurationMS: 140, DiffSingerSteps: 17,
		Method: tts.MethodWaveformSynthesis, Language: "ja",
		RendererSettings: map[string]json.RawMessage{
			"context_duration": json.RawMessage(`false`),
			"mora_duration_ms": json.RawMessage(`123`),
			"diffsinger_steps": json.RawMessage(`5`),
			"custom_option":    json.RawMessage(`"value"`),
		},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ContextDuration == nil || *cfg.ContextDuration || cfg.MoraDurationMS != 123 {
		t.Fatalf("renderer settings did not override fixed fields: %#v", cfg)
	}
	if options.DiffSinger.Steps != 5 {
		t.Fatalf("DiffSinger steps were not overridden: %#v", options.DiffSinger)
	}
	if options.Renderer["custom_option"] != "value" {
		t.Fatalf("unknown renderer setting = %#v", options.Renderer)
	}

	cfg, _, options, err = service.config(Request{Renderer: "test-concat", Method: tts.MethodWaveformSynthesis, Language: "ja", ContextDuration: true, MoraDurationMS: 140, DiffSingerSteps: 17}, false)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ContextDuration == nil || !*cfg.ContextDuration || cfg.MoraDurationMS != 140 {
		t.Fatalf("fixed fields without a map changed: %#v", cfg)
	}
	if options.DiffSinger.Steps != 17 {
		t.Fatalf("typed DiffSinger steps were lost: %#v", options.DiffSinger)
	}
}

func TestResolveRendererSeparatesPublicIDFromProvider(t *testing.T) {
	service := NewService(&plugin.Catalog{Renderers: []plugin.Renderer{
		func() plugin.Renderer {
			renderer := testRenderer("friendly-world", "utautts-world-phrase")
			renderer.DisplayName = "Friendly WORLD"
			return renderer
		}(),
	}}, "", "", "", nil)

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
	service := NewService(&plugin.Catalog{Renderers: []plugin.Renderer{testRenderer("default", "test-concat")}}, "default", "", "", nil)
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
	}}, "", "", "", nil)
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
	}}, "", "", "", testVoicebankResolver{path: "voicebank"})
	if _, _, _, err := service.config(Request{Renderer: "world", Method: tts.MethodWaveformSynthesis, Language: "ja", VoicebankID: "bank"}, true); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unavailable runtime error = %v", err)
	}
}

func TestConfigUsesExplicitModelPath(t *testing.T) {
	service := NewService(&plugin.Catalog{
		Renderers: []plugin.Renderer{testRenderer("test-concat", "test-concat")},
	}, "test-concat", "", "", nil)
	cfg, renderer, _, err := service.config(Request{
		ModelID: "not-in-catalog", ModelPath: "out/working-model.json", Renderer: "test-concat",
		Method: tts.MethodWaveformSynthesis, Language: "ja",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if renderer != "test-concat" || cfg.ProsodyModelPath != "out/working-model.json" {
		t.Fatalf("explicit model path was not preserved: renderer=%q config=%#v", renderer, cfg)
	}
}
