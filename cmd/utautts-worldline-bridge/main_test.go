package main

import (
	"encoding/json"
	"testing"

	"utautts/internal/provider"
)

func TestDecodeProviderManifestUsesCommonUnitJob(t *testing.T) {
	payload, err := json.Marshal(manifest{
		Engine:     "worldline-r-faithful",
		OutputPath: "legacy-output.wav",
		SampleRate: 44100,
		F0Curve:    []float64{220, 220},
		Units:      []unit{{Source: "voice.wav", LengthMS: 100}},
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(provider.UnitRendererJob{
		Version:         provider.UnitRendererJobVersion,
		Contract:        "unit-renderer",
		ContractVersion: 1,
		Plan:            json.RawMessage(`{"version":19}`),
		ProviderPayload: payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeProviderManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.Engine != "worldline-r-faithful" || got.OutputPath != "legacy-output.wav" || len(got.Units) != 1 {
		t.Fatalf("manifest = %#v", got)
	}
}

func TestDecodeProviderManifestKeepsLegacyPayload(t *testing.T) {
	data := []byte(`{"engine":"utautts-world-phrase","output_path":"output.wav","sample_rate":44100,"f0_curve":[220,220],"units":[{"source":"voice.wav","length_ms":100}]}`)
	got, err := decodeProviderManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.Engine != "utautts-world-phrase" || got.OutputPath != "output.wav" {
		t.Fatalf("manifest = %#v", got)
	}
}
