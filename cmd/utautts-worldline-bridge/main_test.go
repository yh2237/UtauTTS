package main

import (
	"encoding/json"
	"testing"

	"utautts/internal/provider"
)

func TestDecodeProviderJobUsesTypedWorldlineOptions(t *testing.T) {
	data, err := json.Marshal(provider.UnitRendererJob{
		Version:         provider.UnitRendererJobVersion,
		Contract:        "unit-renderer",
		ContractVersion: 1,
		Plan:            json.RawMessage(`{"version":19}`),
		Options: provider.UnitRendererOptions{Worldline: &provider.WorldlineOptions{
			Engine: "worldline-r-faithful", SampleRate: 44100,
			F0Curve: []float64{220, 220}, Units: []provider.WorldlineUnit{{Source: "voice.wav", LengthMS: 100}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeProviderJob(data, "job-output.wav")
	if err != nil {
		t.Fatal(err)
	}
	if got.Engine != "worldline-r-faithful" || got.OutputPath != "job-output.wav" || len(got.Units) != 1 {
		t.Fatalf("manifest = %#v", got)
	}
}

func TestDecodeProviderJobRejectsLegacyManifest(t *testing.T) {
	data := []byte(`{"engine":"utautts-world-phrase","output_path":"output.wav","sample_rate":44100,"f0_curve":[220,220],"units":[{"source":"voice.wav","length_ms":100}]}`)
	if _, err := decodeProviderJob(data, "output.wav"); err == nil {
		t.Fatal("legacy manifest was accepted")
	}
}
