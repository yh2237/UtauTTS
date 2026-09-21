package synth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"utautts/internal/engine"
	"utautts/internal/jsut"
	"utautts/internal/plugin"
)

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
		TargetPriorPath: "prior.json", TargetPriorStrength: .5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Config.VoicebankPath != "voicebank" || resolved.Config.Reading != "あ" {
		t.Fatalf("resolved config = %#v", resolved.Config)
	}
	if !resolved.Config.ReleaseSet || resolved.Config.ReleaseMS != 20 ||
		resolved.Config.BoundaryBridgeMS != 12 || resolved.Config.CVVCTiming != "sequential" ||
		resolved.Config.JoinModelPath != "join.json" || resolved.Config.TargetPriorPath != "prior.json" ||
		resolved.Config.TargetPriorStrength != .5 {
		t.Fatalf("CLI settings were not preserved: %#v", resolved.Config)
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

func TestWorldTransitionModelIsEnabledWhenDeclared(t *testing.T) {
	directory := t.TempDir()
	modelPath := filepath.Join(directory, "transition.json")
	if err := os.WriteFile(modelPath, mustTransitionTCNJSON(t), 0o600); err != nil {
		t.Fatal(err)
	}
	bridgePath := filepath.Join(directory, "bridge")
	enginePath := filepath.Join(directory, "world")
	for _, path := range []string{bridgePath, enginePath} {
		if err := os.WriteFile(path, []byte("runtime"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	renderer := testRenderer("world", "utautts-world-phrase")
	renderer.Resources = map[string]plugin.RendererResource{
		"world_engine":           {Path: enginePath, Required: true},
		"worldline_bridge":       {Path: bridgePath, Required: true, Executable: true},
		"world_transition_model": {Path: modelPath},
	}
	service := NewService(&plugin.Catalog{Renderers: []plugin.Renderer{renderer}}, "world", "", "", "", testVoicebankResolver{path: "voicebank"})
	_, _, options, err := service.config(Request{Renderer: "world"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if options.Worldline.TransitionModelPath != modelPath || options.Worldline.TransitionStrength != .20 {
		t.Fatalf("transition options = %#v", options.Worldline)
	}
}

func mustTransitionTCNJSON(t *testing.T) []byte {
	t.Helper()
	layer := func(outputs, inputs, kernel int) jsut.TransitionLayer {
		weight := make([][][]float64, outputs)
		for output := range weight {
			weight[output] = make([][]float64, inputs)
			for input := range weight[output] {
				weight[output][input] = make([]float64, kernel)
			}
		}
		return jsut.TransitionLayer{Weight: weight, Bias: make([]float64, outputs)}
	}
	model := jsut.TransitionTCN{Version: 1, Kind: "jsut_cv_transition_tcn", ID: "test", PositionBins: 3,
		Phones: []string{"a"}, InputSize: 11, HiddenSize: 1, OutputSize: 2, OutputScale: 20,
		Layers: map[string]jsut.TransitionLayer{
			"input": layer(1, 11, 1), "conv1": layer(1, 1, 3), "conv2": layer(1, 1, 3), "output": layer(2, 1, 1),
		}}
	data, err := json.Marshal(model)
	if err != nil {
		t.Fatal(err)
	}
	return data
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
