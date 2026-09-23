package tts

import (
	"math"
	"reflect"
	"testing"

	"utautts/internal/diffsinger"
	"utautts/internal/engine"
	"utautts/internal/plugin"
	"utautts/internal/prosody"
)

func TestDiffSingerIsRegisteredAsNeuralSynthesizer(t *testing.T) {
	synthesizer, found := neuralSynthesizerForProvider(diffsinger.ProviderID)
	if !found || synthesizer.ProviderID() != diffsinger.ProviderID {
		t.Fatalf("DiffSinger neural provider = %#v, found=%v", synthesizer, found)
	}
	if _, found := neuralSynthesizerForProvider("waveform"); found {
		t.Fatal("unit renderer was registered as a neural synthesizer")
	}
}

// 未登録providerは解決されず、登録済みproviderはfactory経由で解決される。
func TestNeuralSynthesizerRegistryResolvesRegisteredFactory(t *testing.T) {
	const id engine.ProviderID = "test-neural-provider"
	RegisterNeuralSynthesizer(id, func() NeuralSynthesizer { return stubNeuralSynthesizer{id: id} })
	defer func() {
		neuralSynthesizersMu.Lock()
		delete(neuralSynthesizers, id)
		neuralSynthesizersMu.Unlock()
	}()
	synthesizer, found := neuralSynthesizerForProvider(id)
	if !found || synthesizer.ProviderID() != id {
		t.Fatalf("registered provider = %#v, found=%v", synthesizer, found)
	}
	if _, found := neuralSynthesizerForProvider("unregistered-neural-provider"); found {
		t.Fatal("unregistered provider must not resolve")
	}
}

type stubNeuralSynthesizer struct{ id engine.ProviderID }

func (s stubNeuralSynthesizer) ProviderID() engine.ProviderID { return s.id }

func (stubNeuralSynthesizer) Synthesize(Config) (*Result, error) { return nil, nil }

func TestDiffSingerUsesSelectedSpeechModel(t *testing.T) {
	model := &prosody.Model{Version: prosody.FramePitchModelVersion, FeatureVersion: 1, Mode: "intonation_frame_tcn_accent_bounded",
		FramePitch: &prosody.FramePitchModel{FeatureNames: []string{"mora_progress"}, InputWeights: [][]float64{{2}}, InputBias: []float64{0}, OutputWeight: []float64{100}, FrameMS: 10, LowCents: -60, HighCents: 60},
	}
	input, err := neuralInputFromConfig(Config{Reading: "あい", ProsodyModel: model, ApplyPitch: true, IntonationStrength: 1, RendererCapabilities: &plugin.Capabilities{FramePitch: true}})
	if err != nil {
		t.Fatal(err)
	}
	if input.PitchCurve == nil {
		t.Fatal("selected model did not reach the neural provider")
	}
	low, high := math.Inf(1), math.Inf(-1)
	for _, value := range input.PitchCurve.Cents {
		low = math.Min(low, value)
		high = math.Max(high, value)
	}
	if high-low < 1 {
		t.Fatal("speech model contour was flattened")
	}
}

func TestDiffSingerSpeechProsodyPreservesManualTimingAndPitch(t *testing.T) {
	cfg := Config{Reading: "あい", Renderer: "diffsinger", MoraDurationMS: 120,
		MoraDurationsMS: []float64{90, 150},
		ManualPitch: &prosody.ManualPitchFile{Version: 1, Reading: "あい", Mode: "replace",
			Points: []prosody.ManualPitchPoint{{Position: 0, Cents: 120}, {Position: 1, Cents: -120}}},
	}
	input, err := neuralInputFromConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(input.MoraDurationsMS, cfg.MoraDurationsMS) {
		t.Fatalf("durations = %v", input.MoraDurationsMS)
	}
	for _, point := range input.PitchPoints {
		if point != 0 {
			t.Fatal("manual pitch leaked into automatic preview")
		}
	}
	if input.PitchCurve == nil || input.PitchCurve.CentsAt(45) <= input.PitchCurve.CentsAt(165) || input.PitchCurve.CentsAt(240) >= 0 {
		t.Fatalf("manual speech curve not reflected at mora positions: %#v", input.PitchCurve)
	}
	cfg.ManualPitch.Reading = "う"
	if _, err := neuralInputFromConfig(cfg); err == nil {
		t.Fatal("accepted mismatched manual reading")
	}
}
