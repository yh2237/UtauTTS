package worldline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"utautts/internal/provider"
)

func TestReadWorldlineBridgeJobReadsCommonUnitJob(t *testing.T) {
	data, err := json.Marshal(provider.UnitRendererJob{
		Version:         provider.UnitRendererJobVersion,
		Contract:        "unit-renderer",
		ContractVersion: 1,
		Plan:            json.RawMessage(`{"version":19}`),
		Options:         provider.UnitRendererOptions{Worldline: &provider.WorldlineOptions{Engine: "utautts-world-phrase"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "job.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadBridgeJob(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Engine != "utautts-world-phrase" {
		t.Fatalf("job = %#v", got)
	}
}

func TestReadWorldlineBridgeJobRejectsOldJobShape(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.json")
	data := []byte(`{"engine":"utautts-world-phrase","output_path":"output.wav"}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadBridgeJob(path); err == nil {
		t.Fatal("old job shape was accepted")
	}
}
