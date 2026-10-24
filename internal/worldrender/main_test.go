package worldrender

import (
	"testing"

	"github.com/yh2237/UtauTTS/internal/provider"
)

func TestManifestFromOptionsCarriesUnits(t *testing.T) {
	got, err := manifestFromOptions(provider.WorldlineOptions{
		Engine: "utautts-world-phrase", SampleRate: 44100,
		F0Curve: []float64{220, 220}, Units: []provider.WorldlineUnit{{Source: "voice.wav", LengthMS: 100}},
	}, "world.dll")
	if err != nil {
		t.Fatal(err)
	}
	if got.Engine != "utautts-world-phrase" || got.WorldEnginePath != "world.dll" || len(got.Units) != 1 {
		t.Fatalf("manifest = %#v", got)
	}
}

func TestManifestFromOptionsRejectsUnknownEngine(t *testing.T) {
	if _, err := manifestFromOptions(provider.WorldlineOptions{Engine: "other"}, ""); err == nil {
		t.Fatal("unknown engine was accepted")
	}
}

func TestManifestFromOptionsKeepsExplicitZeroVolume(t *testing.T) {
	got, err := manifestFromOptions(provider.WorldlineOptions{
		Engine: "utautts-world-phrase", SampleRate: 44100,
		Units: []provider.WorldlineUnit{{Source: "voice.wav", LengthMS: 100, Volume: 0, VolumeSet: true}},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if gain := worldUnitAmplitudeGain(got.Units[0]); gain != 0 {
		t.Fatalf("gain = %g, want 0 for volume 0", gain)
	}
}
