package plugin

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestRepositoryRendererPluginsAreSelfDescribing(t *testing.T) {
	rendererDirectories, _ := DefaultDirectories()
	items, err := DiscoverRenderers(rendererDirectories, func(string) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	if len(items) < 2 {
		t.Fatalf("renderer plugins = %d, want multiple independently described plugins", len(items))
	}
	if items[0].ID != "utautts-world-phrase" {
		t.Fatalf("default renderer = %q, want manifest-priority UtauTTS WORLD phrase", items[0].ID)
	}
	for _, item := range items {
		if item.ID == "" || item.DisplayName == "" || item.Backend == "" || item.Directory == "" {
			t.Fatalf("incomplete renderer plugin: %#v", item)
		}
	}
}

func TestModelUsesIdentityStoredInsideModel(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "arbitrary-filename.json")
	data := []byte(`{
  "id":"intonation.example", "display_name":"Example model", "description":"self described",
  "recommended_renderers":["waveform"], "version":9, "feature_version":1,
  "mode":"standard_japanese_accent", "duration_weights":{},
  "standard_accent":{"frame_ms":10,"accent_range_cents":100,"declination_cents":20,"question_rise_cents":50,"smoothing_ms":20,"p99_cents":200,"max_cents":250},
  "metrics":{}, "training":{}
}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	models, err := DiscoverModels([]string{directory})
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].ID != "intonation.example" || models[0].DisplayName != "Example model" {
		t.Fatalf("models = %#v", models)
	}
	wantHash := fmt.Sprintf("%x", sha256.Sum256(data))
	if models[0].SHA256 != wantHash || models[0].FeatureVersion != 1 {
		t.Fatalf("model provenance = sha256 %q feature %d", models[0].SHA256, models[0].FeatureVersion)
	}
}

func TestModelWithoutIdentityIsNotCatalogued(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "filename-must-not-become-an-id.json")
	data := []byte(`{
  "version":9, "feature_version":1, "mode":"standard_japanese_accent",
  "duration_weights":{},
  "standard_accent":{"frame_ms":10,"accent_range_cents":100,"declination_cents":20,"question_rise_cents":50,"smoothing_ms":20,"p99_cents":200,"max_cents":250},
  "metrics":{}, "training":{}
}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	models, err := DiscoverModels([]string{directory})
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 0 {
		t.Fatalf("identity-free model was catalogued from its filename: %#v", models)
	}
}

func TestTrainingReportBesideModelIsIgnored(t *testing.T) {
	directory := t.TempDir()
	data := []byte(`{"version":1,"model_id":"example","metrics":{"validation":{}}}`)
	if err := os.WriteFile(filepath.Join(directory, "example-training-report.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	models, err := DiscoverModels([]string{directory})
	if err != nil {
		t.Fatalf("training report produced a catalog warning: %v", err)
	}
	if len(models) != 0 {
		t.Fatalf("training report was catalogued as a model: %#v", models)
	}
}

func TestInvalidModelIsReported(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "broken.json"), []byte(`{"version":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := DiscoverModels([]string{directory}); err == nil {
		t.Fatal("invalid model was silently ignored")
	}
}

func TestRepositoryBundlesSelfDescribingModels(t *testing.T) {
	_, modelDirectories := DefaultDirectories()
	models, err := DiscoverModels(modelDirectories)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) == 0 {
		t.Fatal("no bundled self-describing models found")
	}
	if models[0].ID != "frame-intonation-v8" {
		t.Fatalf("default model = %q, want metadata-priority frame-intonation-v8", models[0].ID)
	}
	for _, model := range models {
		if model.ID == "" || model.DisplayName == "" {
			t.Fatalf("incomplete bundled model: %#v", model)
		}
	}
}

func TestWorldlineRenderersDeclareAcceleration(t *testing.T) {
	rendererDirectories, _ := DefaultDirectories()
	items, err := DiscoverRenderers(rendererDirectories, func(string) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"openutau-worldline-r-faithful": "cpu",
		"utautts-world-phrase":          "cpu",
		"utautts-world-phrase-cuda":     "cuda",
	}
	for _, item := range items {
		if acceleration, ok := want[item.ID]; ok {
			if item.Acceleration != acceleration {
				t.Fatalf("renderer %q acceleration = %q, want %q", item.ID, item.Acceleration, acceleration)
			}
			delete(want, item.ID)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing worldline renderers: %#v", want)
	}
}

func TestUnknownRendererFallsBackToDefault(t *testing.T) {
	catalog := &Catalog{Renderers: []Renderer{{ID: "default"}, {ID: "other"}}}
	for _, requested := range []string{"unknown", "removed-renderer", ""} {
		got, ok := catalog.Renderer(requested)
		if !ok || got.ID != "default" {
			t.Fatalf("Renderer(%q) = %#v, %v", requested, got, ok)
		}
	}
}

func TestExternalResamplerOptionsAreValidated(t *testing.T) {
	velocity, modulation := 86, 4
	valid := Renderer{
		ManifestVersion: ManifestVersion, Kind: "renderer", ID: "external", DisplayName: "External",
		Backend:          "utau-external-resampler",
		ResamplerOptions: &ResamplerOptions{Velocity: &velocity, Flags: "g-3Mt10", Modulation: &modulation, Tempo: 120},
	}
	if err := validateRenderer(valid, func(string) bool { return true }); err != nil {
		t.Fatalf("valid resampler options rejected: %v", err)
	}
	invalidVelocity := 201
	valid.ResamplerOptions.Velocity = &invalidVelocity
	if err := validateRenderer(valid, func(string) bool { return true }); err == nil {
		t.Fatal("out-of-range resampler velocity was accepted")
	}
	valid.ResamplerOptions.Velocity = &velocity
	valid.ResamplerOptions.Flags = "g-3 bad"
	if err := validateRenderer(valid, func(string) bool { return true }); err == nil {
		t.Fatal("resampler flags containing whitespace were accepted")
	}
}

func TestPackagedDirectoriesTakePrecedenceOverWorkspaceDirectories(t *testing.T) {
	workspace := t.TempDir()
	packaged := filepath.Join(workspace, "release", "UtauTTS")
	if err := os.MkdirAll(filepath.Join(workspace, "plugins", "renderers"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, "models"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(packaged, "plugins", "renderers"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(packaged, "models"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "go.mod"), []byte("module test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	renderers, models := defaultDirectories(filepath.Join(packaged, "tools", "utautts-cli.exe"), workspace)
	if len(renderers) != 1 || filepath.Clean(renderers[0]) != filepath.Join(packaged, "plugins", "renderers") {
		t.Fatalf("renderer directories = %#v", renderers)
	}
	if len(models) != 1 || filepath.Clean(models[0]) != filepath.Join(packaged, "models") {
		t.Fatalf("model directories = %#v", models)
	}
}
