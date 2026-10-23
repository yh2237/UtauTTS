package plugin

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestModelUsesIdentityStoredInsideModel(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "arbitrary-filename.json")
	data := []byte(`{
  "id":"intonation.example", "display_name":"Example model", "description":"self described",
  "recommended_renderers":["test-concat"], "version":8, "feature_version":1,
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
	models, err := DiscoverModels(DefaultModelDirectories())
	if err != nil {
		t.Fatal(err)
	}
	if len(models) == 0 {
		t.Fatal("no bundled self-describing models found")
	}
	if models[0].ID != "frame-intonation-tcn-v12" {
		t.Fatalf("default model = %q, want metadata-priority frame-intonation-tcn-v12", models[0].ID)
	}
	for _, model := range models {
		if model.ID == "" || model.DisplayName == "" {
			t.Fatalf("incomplete bundled model: %#v", model)
		}
	}
	for _, model := range models {
		if model.Language == "en" {
			t.Fatalf("English intonation model is still bundled: %q", model.ID)
		}
	}
}

func TestPackagedDirectoriesTakePrecedenceOverWorkspaceDirectories(t *testing.T) {
	workspace := t.TempDir()
	packaged := filepath.Join(workspace, "release", "UtauTTS")
	if err := os.MkdirAll(filepath.Join(workspace, "models"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(packaged, "models"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "go.mod"), []byte("module test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	models := defaultModelDirectories(filepath.Join(packaged, "tools", "utautts-cli.exe"), workspace)
	if len(models) != 1 || filepath.Clean(models[0]) != filepath.Join(packaged, "models") {
		t.Fatalf("model directories = %#v", models)
	}
}
