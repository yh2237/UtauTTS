package synth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"utautts/internal/render"
	"utautts/internal/tts"
)

func TestRendererManifestSettingDefaultsMatchResolver(t *testing.T) {
	for _, rendererID := range []string{"utautts-world-phrase", "classic-utau", "diffsinger"} {
		t.Run(rendererID, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "renderer", rendererID, "renderer.json"))
			if err != nil {
				t.Fatal(err)
			}
			var manifest struct {
				Settings []struct {
					ID      string          `json:"id"`
					Default json.RawMessage `json:"default"`
				} `json:"settings"`
			}
			if err := json.Unmarshal(data, &manifest); err != nil {
				t.Fatal(err)
			}
			for _, setting := range manifest.Settings {
				switch setting.ID {
				case "context_duration", "context_duration_strength",
					"boundary_tone_strength", "stretch_adapt_strength", "pause_context_strength":
					t.Errorf("internal setting %q is exposed in the renderer UI", setting.ID)
				}
				spec, ok := rendererSettingSpecFor(rendererSettingSpecs, setting.ID)
				if !ok {
					continue
				}
				var declared any
				if err := json.Unmarshal(setting.Default, &declared); err != nil {
					t.Fatalf("setting %q default: %v", setting.ID, err)
				}
				if !reflect.DeepEqual(declared, spec.defaultValue) {
					t.Errorf("setting %q default = %v, resolver = %v", setting.ID, declared, spec.defaultValue)
				}
			}
		})
	}
}

func TestWorldlineSettingPrecedence(t *testing.T) {
	resolve := func(request Request, enabled func(render.WorldlineProviderOptions) bool) bool {
		var cfg tts.Config
		options := render.ProviderOptions{Worldline: request.Worldline}
		resolveRendererSettings(request, &cfg, &options)
		return enabled(options.Worldline)
	}
	off := false
	tests := []struct {
		id      string
		enabled func(render.WorldlineProviderOptions) bool
	}{
		{"timing_warp", render.WorldlineProviderOptions.TimingWarpEnabled},
		{"microprosody", render.WorldlineProviderOptions.MicroprosodyEnabled},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			if !resolve(Request{}, test.enabled) {
				t.Fatalf("%s should default to on", test.id)
			}
			disabled := render.WorldlineProviderOptions{TimingWarp: &off, Microprosody: &off}
			if resolve(Request{Worldline: disabled}, test.enabled) {
				t.Fatalf("typed %s=false should disable it", test.id)
			}
			settings := map[string]json.RawMessage{test.id: json.RawMessage("true")}
			if !resolve(Request{Worldline: disabled, RendererSettings: settings}, test.enabled) {
				t.Fatalf("renderer_settings should override the typed field for %s", test.id)
			}
			settings = map[string]json.RawMessage{test.id: json.RawMessage("false")}
			if resolve(Request{RendererSettings: settings}, test.enabled) {
				t.Fatalf("renderer_settings false should disable %s", test.id)
			}
		})
	}
}
