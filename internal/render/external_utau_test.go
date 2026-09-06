package render

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"utautts/internal/audio"
	"utautts/internal/plan"
)

func TestMain(m *testing.M) {
	if os.Getenv("UTAUTTS_TEST_RESAMPLER") == "1" {
		if len(os.Args) == 16 {
			if capture := os.Getenv("UTAUTTS_TEST_WAVTOOL_ARGS"); capture != "" {
				data, err := json.Marshal(os.Args[1:])
				if err != nil || os.WriteFile(capture, data, 0o600) != nil {
					os.Exit(6)
				}
			}
			data, err := os.ReadFile(os.Args[2])
			if err != nil || os.WriteFile(os.Args[1], data, 0o600) != nil {
				os.Exit(7)
			}
			os.Exit(0)
		}
		if len(os.Args) != 14 {
			os.Exit(2)
		}
		if capture := os.Getenv("UTAUTTS_TEST_RESAMPLER_ARGS"); capture != "" {
			data, err := json.Marshal(os.Args[1:])
			if err != nil || os.WriteFile(capture, data, 0o600) != nil {
				os.Exit(5)
			}
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

func TestResolveResamplerExpressionsOverridesOnePosition(t *testing.T) {
	velocity, volume, flags, modulation, tempo := 86, 72, "g-3", 4, 150.0
	values, err := resolveResamplerExpressions([]ResamplerExpression{{
		Position: 2, Velocity: &velocity, Volume: &volume, Flags: &flags, Modulation: &modulation, Tempo: &tempo,
	}}, effectiveResamplerExpression{velocity: 100, tempo: 120})
	if err != nil {
		t.Fatal(err)
	}
	if got := values.get(1); got.velocity != 100 || got.tempo != 120 {
		t.Fatalf("default expression = %#v", got)
	}
	if got := values.get(2); got.velocity != 86 || got.volume == nil || *got.volume != 72 || got.flags != "g-3" || got.modulation != 4 || got.tempo != 150 {
		t.Fatalf("overridden expression = %#v", got)
	}
}

func TestUtauResamplerArgumentsMatchOpenUtauClassicContract(t *testing.T) {
	got := (utauResamplerArguments{
		input: "in.wav", output: "out.wav", tone: 61, velocity: 86, flags: "g-3Mt10",
		offsetMS: 12.5, requiredMS: 250, consonantMS: 80.25, cutoffMS: -120,
		volume: 73, modulation: 4, tempo: 120, pitches: []int{0, 0, 1, -1},
	}).commandLine()
	want := []string{
		"in.wav", "out.wav", "C#4", "86", "g-3Mt10", "12.500000", "250.000000",
		"80.250000", "-120.000000", "73", "4", "!120.000000", "AA#1#AB//",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("classic resampler arguments = %#v, want %#v", got, want)
	}
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

func TestUtauPitchIntervalUsesConfiguredTempo(t *testing.T) {
	if got, want := utauPitchIntervalMS(120), 60000.0/120.0*5.0/480.0; math.Abs(got-want) > 1e-12 {
		t.Fatalf("120 BPM interval = %.12f, want %.12f", got, want)
	}
	if got, want := utauPitchIntervalMS(150), 60000.0/150.0*5.0/480.0; math.Abs(got-want) > 1e-12 {
		t.Fatalf("150 BPM interval = %.12f, want %.12f", got, want)
	}
}

func TestRenderUtauExternalResamplerInvokesCompatibleExecutable(t *testing.T) {
	t.Setenv("UTAUTTS_TEST_RESAMPLER", "1")
	directory := t.TempDir()
	capture := filepath.Join(directory, "arguments.json")
	t.Setenv("UTAUTTS_TEST_RESAMPLER_ARGS", capture)
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
		ProviderOptions: ProviderOptions{Classic: ClassicOptions{ResamplerPath: executable}},
		ReleaseSet:      true, CVVCTiming: CVVCTimingLegacy,
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
	encoded, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	var arguments []string
	if err := json.Unmarshal(encoded, &arguments); err != nil {
		t.Fatal(err)
	}
	if len(arguments) != 13 {
		t.Fatalf("resampler argument count = %d, want 13: %#v", len(arguments), arguments)
	}
	if arguments[0] != source || arguments[2] != "A3" || arguments[3] != "100" || arguments[4] != "" {
		t.Fatalf("unexpected OpenUtau-compatible leading arguments: %#v", arguments[:5])
	}
	if arguments[5] != "0.000000" || arguments[7] != "40.000000" || arguments[9] != "100" || arguments[10] != "0" || arguments[11] != "!120.000000" {
		t.Fatalf("unexpected OpenUtau-compatible timing arguments: %#v", arguments[5:12])
	}
	if arguments[12] == "" {
		t.Fatal("resampler pitch argument is empty")
	}
}

func TestRenderUtauExternalResamplerUsesExternalWavtool(t *testing.T) {
	t.Setenv("UTAUTTS_TEST_RESAMPLER", "1")
	directory := t.TempDir()
	capture := filepath.Join(directory, "wavtool-arguments.json")
	t.Setenv("UTAUTTS_TEST_WAVTOOL_ARGS", capture)
	source := filepath.Join(directory, "source.wav")
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
	p := &plan.Plan{DurationMS: 120, Units: []plan.Unit{{
		Position: 0, Role: "mora", Mora: "あ", Alias: "あ", Source: source,
		DurationMS: 120, ConsonantMS: 40, PreutteranceMS: 60, OverlapMS: 20,
		PitchFactor: 1, EnergyFactor: 1,
	}}}
	result, err := renderUtauExternalResampler(p, Config{
		ProviderOptions: ProviderOptions{Classic: ClassicOptions{ResamplerPath: executable, WavtoolPath: executable}},
		ReleaseSet:      true, CVVCTiming: CVVCTimingLegacy,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Data) == 0 {
		t.Fatal("external wavtool returned no audio")
	}
	encoded, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	var arguments []string
	if err := json.Unmarshal(encoded, &arguments); err != nil {
		t.Fatal(err)
	}
	if len(arguments) != 15 || arguments[3] != "115.200000@120.000000+60.000000" || arguments[4] != "0.000000" {
		t.Fatalf("wavtool arguments = %#v", arguments)
	}
}

func TestFinalizeExternalWavtoolOutputCombinesWhdAndDat(t *testing.T) {
	output := filepath.Join(t.TempDir(), "phrase.wav")
	if err := os.WriteFile(output+".whd", []byte("header"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output+".dat", []byte("samples"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := finalizeExternalWavtoolOutput(output); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "headersamples"; got != want {
		t.Fatalf("combined output = %q, want %q", got, want)
	}
}
