package synth

import (
	"encoding/json"
	"testing"

	"github.com/yh2237/UtauTTS/internal/plugin"
	"github.com/yh2237/UtauTTS/internal/tts"
)

func boolPointer(value bool) *bool { return &value }

type testVoicebankResolver struct{ path string }

func TestDefaultRequestResolvesSynthesisSettings(t *testing.T) {
	service := NewService(&plugin.Catalog{}, "", "", testVoicebankResolver{path: "voicebank"})
	request := DefaultRequest()
	request.Method = tts.MethodWaveformSynthesis
	request.Language = "ja"
	request.VoicebankID = "bank"
	resolved, err := service.ResolveSynthesis(request)
	if err != nil {
		t.Fatal(err)
	}
	cfg := resolved.Config
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
	if !cfg.World.TimingWarpEnabled() || !cfg.World.MicroprosodyEnabled() || len(resolved.Diagnostics) != 0 {
		t.Fatalf("default WORLD options = %#v, diagnostics = %v", cfg.World, resolved.Diagnostics)
	}
}

func (resolver testVoicebankResolver) Resolve(string) (string, bool) {
	return resolver.path, true
}

func TestResolveSynthesisUsesDirectVoicebankPathAndNormalizesKana(t *testing.T) {
	service := NewService(&plugin.Catalog{}, "", "", nil)
	resolved, err := service.ResolveSynthesis(Request{
		VoicebankPath: "voicebank", Kana: "あ", Method: tts.MethodWaveformSynthesis, Language: "ja",
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

func TestApplySettingsKnownIDs(t *testing.T) {
	cfg := tts.Config{MoraDurationMS: 140, ContextDuration: boolPointer(true)}
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
	}
	if diagnostics := applySettingsMap(settingSpecs, settings, &cfg); len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diagnostics)
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
}

func TestApplySettingsOutOfRangeIsIgnored(t *testing.T) {
	cfg := tts.Config{MoraDurationMS: 140, PauseDurationMS: 180}
	diagnostics := applySettingsMap(settingSpecs, map[string]json.RawMessage{
		"mora_duration_ms":  json.RawMessage(`100000000`),
		"pause_duration_ms": json.RawMessage(`-1`),
	}, &cfg)
	if cfg.MoraDurationMS != 140 || cfg.PauseDurationMS != 180 {
		t.Fatalf("out-of-range values changed config: %#v", cfg)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
}

func TestApplySettingsTypeMismatchIsIgnored(t *testing.T) {
	cfg := tts.Config{MoraDurationMS: 140, ContextDuration: boolPointer(true)}
	diagnostics := applySettingsMap(settingSpecs, map[string]json.RawMessage{
		"mora_duration_ms":    json.RawMessage(`true`),
		"context_duration":    json.RawMessage(`"yes"`),
		"intonation_strength": json.RawMessage(`"high"`),
	}, &cfg)
	if cfg.MoraDurationMS != 140 || cfg.ContextDuration == nil || !*cfg.ContextDuration {
		t.Fatalf("mismatched values changed config: %#v", cfg)
	}
	if len(diagnostics) != 3 {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
}

func TestConfigSettingsOverrideFixedFields(t *testing.T) {
	service := NewService(&plugin.Catalog{}, "", "", nil)
	cfg, diagnostics, err := service.config(Request{
		ContextDuration: true, MoraDurationMS: 140,
		Method: tts.MethodWaveformSynthesis, Language: "ja",
		Settings: map[string]json.RawMessage{
			"context_duration": json.RawMessage(`false`),
			"mora_duration_ms": json.RawMessage(`123`),
			"custom_option":    json.RawMessage(`"value"`),
		},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ContextDuration == nil || *cfg.ContextDuration || cfg.MoraDurationMS != 123 {
		t.Fatalf("settings did not override fixed fields: %#v", cfg)
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != "setting-ignored" {
		t.Fatalf("unknown setting diagnostics = %v", diagnostics)
	}

	cfg, _, err = service.config(Request{Method: tts.MethodWaveformSynthesis, Language: "ja", ContextDuration: true, MoraDurationMS: 140}, false)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ContextDuration == nil || !*cfg.ContextDuration || cfg.MoraDurationMS != 140 {
		t.Fatalf("fixed fields without a map changed: %#v", cfg)
	}
}

func TestConfigUsesExplicitModelPath(t *testing.T) {
	service := NewService(&plugin.Catalog{}, "", "", nil)
	cfg, _, err := service.config(Request{
		ModelID: "not-in-catalog", ModelPath: "out/working-model.json",
		Method: tts.MethodWaveformSynthesis, Language: "ja",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ProsodyModelPath != "out/working-model.json" {
		t.Fatalf("explicit model path was not preserved: %#v", cfg)
	}
}
