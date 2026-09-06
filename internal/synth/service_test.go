package synth

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"utautts/internal/engine"
	"utautts/internal/plugin"
)

type testVoicebankResolver struct{ path string }

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
		Renderers:  []plugin.Renderer{{ID: "classic-utau", Backend: "utau-external-resampler"}},
		Resamplers: []plugin.ClassicTool{{ID: "nested/resampler.exe", Path: resamplerPath}},
		Wavtools:   []plugin.ClassicTool{{ID: "builtin", BuiltIn: true}, {ID: "wavtool.exe", Path: wavtoolPath}},
	}
	service := NewService(catalog, "classic-utau", "", "", "", "", testVoicebankResolver{path: "voicebank"})
	cfg, renderer, err := service.config(Request{
		Renderer: "classic-utau", Resampler: "nested/resampler.exe", Wavtool: "wavtool.exe",
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if renderer != "classic-utau" || cfg.ExternalResamplerPath != resamplerPath || cfg.ExternalWavtoolPath != wavtoolPath {
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

func TestClassicUtauRejectsMissingToolExecutable(t *testing.T) {
	catalog := &plugin.Catalog{
		Renderers:  []plugin.Renderer{{ID: "classic-utau", Backend: "utau-external-resampler"}},
		Resamplers: []plugin.ClassicTool{{ID: "missing.exe", Path: filepath.Join(t.TempDir(), "missing.exe")}},
		Wavtools:   []plugin.ClassicTool{{ID: "builtin", BuiltIn: true}},
	}
	service := NewService(catalog, "classic-utau", "", "", "", "", testVoicebankResolver{path: "voicebank"})
	if _, err := service.ResolveClassicTools("missing.exe", "builtin"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing tool error = %v", err)
	}
}

func TestResolveRendererSeparatesPublicIDFromProvider(t *testing.T) {
	service := NewService(&plugin.Catalog{Renderers: []plugin.Renderer{{
		ID: "friendly-world", DisplayName: "Friendly WORLD", Backend: "utautts-world-phrase",
	}}}, "", "", "", "", "", nil)

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
	service := NewService(&plugin.Catalog{Renderers: []plugin.Renderer{{
		ID: "default", DisplayName: "Default", Backend: "waveform",
	}}}, "default", "", "", "", "", nil)
	if _, err := service.ResolveRenderer("removed"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing explicit renderer error = %v", err)
	}
}

func TestRendererAvailabilityReportsMissingRuntimeWithoutRejectingListing(t *testing.T) {
	service := NewService(&plugin.Catalog{Renderers: []plugin.Renderer{{
		ID: "world", DisplayName: "WORLD", Backend: "utautts-world-phrase",
	}}}, "", "", "", "", "", nil)
	availability := service.RendererAvailability()["world"]
	if availability.Available || len(availability.Issues) == 0 {
		t.Fatalf("availability = %#v", availability)
	}
}

func TestSynthesisConfigRejectsUnavailableRendererRuntime(t *testing.T) {
	service := NewService(&plugin.Catalog{Renderers: []plugin.Renderer{{
		ID: "world", DisplayName: "WORLD", Backend: "utautts-world-phrase",
	}}}, "", "", "", "", "", testVoicebankResolver{path: "voicebank"})
	if _, _, err := service.config(Request{Renderer: "world"}, true); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unavailable runtime error = %v", err)
	}
}
