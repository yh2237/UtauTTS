package render

import (
	"math"
	"os"
	"testing"

	"utautts/internal/audio"
	"utautts/internal/plan"
)

func TestMain(m *testing.M) {
	if os.Getenv("UTAUTTS_TEST_RESAMPLER") == "1" {
		if len(os.Args) < 14 {
			os.Exit(2)
		}
		data, err := os.ReadFile(os.Args[1])
		if err != nil {
			os.Exit(3)
		}
		if err := os.WriteFile(os.Args[2], data, 0o600); err != nil {
			os.Exit(4)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestEncodeUtauPitch(t *testing.T) {
	if got, want := encodeUtauPitch([]int{0, 0, 0, 1, -1}), "AA#2#AB//"; got != want {
		t.Fatalf("encodeUtauPitch() = %q, want %q", got, want)
	}
}

func TestEncodeUtauPitchClampsToSigned12Bit(t *testing.T) {
	if got, want := encodeUtauPitch([]int{-9999, 9999}), "gAf/"; got != want {
		t.Fatalf("encodeUtauPitch() = %q, want %q", got, want)
	}
}

func TestMidiToneName(t *testing.T) {
	for tone, want := range map[int]string{0: "C-1", 60: "C4", 61: "C#4", 69: "A4", 127: "G9"} {
		if got := midiToneName(tone); got != want {
			t.Errorf("midiToneName(%d) = %q, want %q", tone, got, want)
		}
	}
}

func TestRenderUtauExternalResamplerInvokesCompatibleExecutable(t *testing.T) {
	t.Setenv("UTAUTTS_TEST_RESAMPLER", "1")
	directory := t.TempDir()
	source := directory + string(os.PathSeparator) + "source.wav"
	data := make([]int16, 44100*3/10)
	for index := range data {
		data[index] = int16(math.Sin(2*math.Pi*220*float64(index)/44100) * 12000)
	}
	if err := audio.WriteWav(source, &audio.PCM{SampleRate: 44100, Channels: 1, Data: data}); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	synthesisPlan := &plan.Plan{DurationMS: 120, Units: []plan.Unit{{
		Position: 0, Role: "mora", Mora: "あ", Alias: "あ", Source: source,
		DurationMS: 120, ConsonantMS: 40, PreutteranceMS: 60, OverlapMS: 20,
		PitchFactor: 1, EnergyFactor: 1,
	}}}
	result, err := renderUtauExternalResampler(synthesisPlan, Config{
		ExternalResamplerPath: executable, ReleaseSet: true, CVVCTiming: CVVCTimingLegacy,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.SampleRate != 44100 || result.Channels != 1 || len(result.Data) == 0 {
		t.Fatalf("unexpected output: %#v", result)
	}
	nonzero := false
	for _, sample := range result.Data {
		if sample != 0 {
			nonzero = true
			break
		}
	}
	if !nonzero {
		t.Fatal("external renderer output is silent")
	}
}
