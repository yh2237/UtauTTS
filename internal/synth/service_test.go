package synth

import (
	"testing"

	"utautts/internal/plugin"
)

type testVoicebankResolver struct{ path string }

func (resolver testVoicebankResolver) Resolve(string) (string, bool) {
	return resolver.path, true
}

func TestClassicUtauResolvesSelectedTools(t *testing.T) {
	catalog := &plugin.Catalog{
		Renderers:  []plugin.Renderer{{ID: "classic-utau", Backend: "utau-external-resampler"}},
		Resamplers: []plugin.ClassicTool{{ID: "nested/resampler.exe", Path: "resampler-path"}},
		Wavtools:   []plugin.ClassicTool{{ID: "builtin", BuiltIn: true}, {ID: "wavtool.exe", Path: "wavtool-path"}},
	}
	service := NewService(catalog, "classic-utau", "", "", "", "", testVoicebankResolver{path: "voicebank"})
	cfg, renderer, err := service.config(Request{
		Renderer: "classic-utau", Resampler: "nested/resampler.exe", Wavtool: "wavtool.exe",
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if renderer != "classic-utau" || cfg.ExternalResamplerPath != "resampler-path" || cfg.ExternalWavtoolPath != "wavtool-path" {
		t.Fatalf("classic UTAU config = %#v, renderer %q", cfg, renderer)
	}
}

func TestClassicUtauRejectsUnknownResampler(t *testing.T) {
	catalog := &plugin.Catalog{
		Renderers: []plugin.Renderer{{ID: "classic-utau", Backend: "utau-external-resampler"}},
		Wavtools:  []plugin.ClassicTool{{ID: "builtin", BuiltIn: true}},
	}
	service := NewService(catalog, "classic-utau", "", "", "", "", testVoicebankResolver{path: "voicebank"})
	if _, _, err := service.config(Request{Renderer: "classic-utau", Resampler: "missing", Wavtool: "builtin"}, true); err == nil {
		t.Fatal("unknown resampler was accepted")
	}
}
