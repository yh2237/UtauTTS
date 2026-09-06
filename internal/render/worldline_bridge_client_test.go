package render

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"utautts/internal/plan"
	"utautts/internal/provider"
)

func TestReadWorldlineBridgeJobReadsCommonUnitJob(t *testing.T) {
	data, err := json.Marshal(provider.UnitRendererJob{
		Version:         provider.UnitRendererJobVersion,
		Contract:        "unit-renderer",
		ContractVersion: 1,
		Plan:            json.RawMessage(`{"version":19}`),
		Options:         provider.UnitRendererOptions{Worldline: &provider.WorldlineOptions{Engine: "worldline-r-faithful"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "job.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readWorldlineBridgeJob(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Engine != "worldline-r-faithful" {
		t.Fatalf("job = %#v", got)
	}
}

func TestReadWorldlineBridgeJobRejectsLegacyManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.json")
	data := []byte(`{"engine":"utautts-world-phrase","output_path":"output.wav"}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readWorldlineBridgeJob(path); err == nil {
		t.Fatal("legacy manifest was accepted")
	}
}

func TestWorldlineProviderJobCarriesCommonPlanAndResources(t *testing.T) {
	synthesisPlan := &plan.Plan{Version: plan.Version, Voicebank: "bank", Units: []plan.Unit{{Source: "voice.wav", DurationMS: 100}}}
	job, err := worldlineProviderJob(synthesisPlan, Config{ApplyPitch: true}, worldlineManifest{
		Engine: "worldline-r-faithful", WorldlinePath: "worldline.dll", OutputPath: "output.wav",
		SampleRate: 44100, F0Curve: []float64{220, 220},
	}, "bridge.exe")
	if err != nil {
		t.Fatal(err)
	}
	if job.Version != provider.UnitRendererJobVersion || job.Contract != "unit-renderer" ||
		len(job.Plan) == 0 || job.Resources["worldline"] != "worldline.dll" ||
		job.Resources["worldline_bridge"] != "bridge.exe" || !job.Options.ApplyPitch ||
		job.Options.Worldline == nil {
		t.Fatalf("job = %#v", job)
	}
	if job.Options.Worldline.Engine != "worldline-r-faithful" || job.Options.Worldline.SampleRate != 44100 {
		t.Fatalf("worldline options = %#v", job.Options.Worldline)
	}
}
