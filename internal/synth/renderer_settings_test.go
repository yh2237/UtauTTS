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

func TestTimingWarpSettingPrecedence(t *testing.T) {
	resolve := func(request Request) bool {
		var cfg tts.Config
		options := render.ProviderOptions{Worldline: request.Worldline}
		resolveRendererSettings(request, &cfg, &options)
		return options.Worldline.TimingWarpEnabled()
	}
	off := false
	if !resolve(Request{}) {
		t.Fatal("timing warp should default to on")
	}
	if resolve(Request{Worldline: render.WorldlineProviderOptions{TimingWarp: &off}}) {
		t.Fatal("typed Worldline.TimingWarp=false should disable timing warp")
	}
	settings := map[string]json.RawMessage{"timing_warp": json.RawMessage("true")}
	if !resolve(Request{Worldline: render.WorldlineProviderOptions{TimingWarp: &off}, RendererSettings: settings}) {
		t.Fatal("renderer_settings should override the typed field")
	}
	settings = map[string]json.RawMessage{"timing_warp": json.RawMessage("false")}
	if resolve(Request{RendererSettings: settings}) {
		t.Fatal("renderer_settings false should disable timing warp")
	}
}
