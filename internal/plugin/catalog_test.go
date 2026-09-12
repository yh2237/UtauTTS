package plugin

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRepositoryRendererPluginsAreSelfDescribing(t *testing.T) {
	directories, _ := DefaultDirectories()
	items, err := DiscoverRenderers(directories, func(string) bool { return true })
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
		if item.ID == "" || item.DisplayName == "" || item.Provider == "" {
			t.Fatalf("incomplete renderer plugin: %#v", item)
		}
		if item.ManifestVersion != 2 || item.Contract == "" || item.ProviderVersion == "" {
			t.Fatalf("bundled renderer was not migrated to explicit v2 metadata: %#v", item)
		}
	}
}

func TestModelUsesIdentityStoredInsideModel(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "arbitrary-filename.json")
	data := []byte(`{
  "id":"intonation.example", "display_name":"Example model", "description":"self described",
  "recommended_renderers":["waveform"], "version":8, "feature_version":1,
  "mode":"intonation_frame_tcn_accent_bounded", "duration_weights":{},
  "frame_pitch":{"feature_names":["bias"],"input_weights":[[0]],"input_bias":[0],"layers":[],"output_weight":[0],"output_bias":0,"frame_ms":10,"low_cents":-100,"high_cents":100},
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
  "version":8, "feature_version":1, "mode":"intonation_frame_tcn_accent_bounded",
  "duration_weights":{},
  "frame_pitch":{"frame_ms":10,"low_cents":-100,"high_cents":100},
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
	var english *Model
	for index := range models {
		if models[index].ID == "english-intonation-v1" {
			english = &models[index]
			break
		}
	}
	if english == nil || english.Language != "en" || !english.FrameContour || english.RequiresFeatures {
		t.Fatalf("English model metadata = %#v", english)
	}
}

func TestWorldlineRenderersDeclareAcceleration(t *testing.T) {
	directories, _ := DefaultDirectories()
	items, err := DiscoverRenderers(directories, func(string) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"utautts-world-phrase": "cpu",
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

func TestUnknownRendererRequiresExplicitSelection(t *testing.T) {
	catalog := &Catalog{Renderers: []Renderer{{ID: "default"}, {ID: "other"}}}
	for _, requested := range []string{"unknown", "removed-renderer", ""} {
		got, ok := catalog.Renderer(requested)
		if (requested == "" && (!ok || got.ID != "default")) || (requested != "" && ok) {
			t.Fatalf("Renderer(%q) = %#v, %v", requested, got, ok)
		}
	}
}

func TestPackagedDirectoriesTakePrecedenceOverWorkspaceDirectories(t *testing.T) {
	workspace := t.TempDir()
	packaged := filepath.Join(workspace, "release", "UtauTTS")
	if err := os.MkdirAll(filepath.Join(workspace, "renderer"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, "models"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(packaged, "renderer"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(packaged, "models"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "go.mod"), []byte("module test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	renderers, models := defaultDirectories(filepath.Join(packaged, "tools", "utautts-cli.exe"), workspace)
	if len(renderers) != 1 || filepath.Clean(renderers[0]) != filepath.Join(packaged, "renderer") {
		t.Fatalf("renderer directories = %#v", renderers)
	}
	if len(models) != 1 || filepath.Clean(models[0]) != filepath.Join(packaged, "models") {
		t.Fatalf("model directories = %#v", models)
	}
}

func TestDiscoverClassicToolsUsesRelativeIDsAndIgnoresLibraries(t *testing.T) {
	resamplers := filepath.Join(t.TempDir(), "Resamplers")
	wavtools := filepath.Join(t.TempDir(), "Wavtools")
	if err := os.MkdirAll(filepath.Join(resamplers, "L2R"), 0o755); err != nil {
		t.Fatal(err)
	}
	executableName := "L2R"
	if runtime.GOOS == "windows" {
		executableName += ".exe"
	}
	for path, data := range map[string]string{
		filepath.Join(resamplers, "L2R", executableName): "exe",
		filepath.Join(resamplers, "L2R", "runtime.dll"):  "dll",
		filepath.Join(wavtools, executableName):          "exe",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	gotResamplers, gotWavtools := DiscoverClassicTools([]string{resamplers}, []string{wavtools})
	if len(gotResamplers) != 1 || gotResamplers[0].ID != "L2R/"+executableName {
		t.Fatalf("resamplers = %#v", gotResamplers)
	}
	if len(gotWavtools) != 1 || gotWavtools[0].ID != executableName {
		t.Fatalf("wavtools = %#v", gotWavtools)
	}
}

func TestClassicToolLookupIsCaseInsensitive(t *testing.T) {
	catalog := Catalog{Resamplers: []ClassicTool{{ID: "L2R/L2R.exe", Path: "resampler"}}}
	tool, ok := catalog.Resampler("l2r/l2r.EXE")
	if !ok || tool.Path != "resampler" {
		t.Fatalf("Resampler() = %#v, %v", tool, ok)
	}
}

func TestDefaultCatalogIncludesClassicUtau(t *testing.T) {
	catalog, err := DiscoverWithDefaults(nil, nil, func(string) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	renderer, ok := catalog.Renderer("classic-utau")
	if !ok || renderer.Provider != "utau-external-resampler" {
		t.Fatalf("classic renderer = %#v, %v", renderer, ok)
	}
	wavtool, ok := catalog.Wavtool("builtin")
	if !ok || !wavtool.BuiltIn || wavtool.Path != "" {
		t.Fatalf("built-in wavtool = %#v, %v", wavtool, ok)
	}
}

func TestExplicitRendererDirectoryOverridesPackagedID(t *testing.T) {
	root := t.TempDir()
	packaged := filepath.Join(root, "packaged")
	explicit := filepath.Join(root, "explicit")
	for _, directory := range []string{
		filepath.Join(packaged, "waveform"), filepath.Join(explicit, "waveform"),
	} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	manifest := `{"manifest_version":2,"kind":"synthesis-engine","id":"waveform","display_name":"packaged","contract":"unit-renderer","provider":"waveform","provider_version":"1"}`
	if err := os.WriteFile(filepath.Join(packaged, "waveform", "renderer.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest = `{"manifest_version":2,"kind":"synthesis-engine","id":"waveform","display_name":"explicit","contract":"unit-renderer","provider":"waveform","provider_version":"1"}`
	if err := os.WriteFile(filepath.Join(explicit, "waveform", "renderer.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	items, err := DiscoverRenderers([]string{explicit, packaged}, func(string) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].DisplayName != "explicit" {
		t.Fatalf("renderer override = %#v", items)
	}
}

func TestDirectoryWithoutManifestHasNoRenderers(t *testing.T) {
	items, err := DiscoverRenderers([]string{t.TempDir()}, func(string) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("manifest-free directory yielded renderers: %#v", items)
	}
}
