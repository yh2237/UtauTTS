package tts

import (
	"reflect"
	"testing"

	"utautts/internal/frontend"
	"utautts/internal/plugin"
	"utautts/internal/prosody"
)

func japaneseSpeechTestPredictions(count int) []prosody.Prediction {
	predictions := make([]prosody.Prediction, count)
	for i := range predictions {
		predictions[i] = prosody.Prediction{DurationFactor: 1, PitchFactor: 1, EnergyFactor: 1}
	}
	return predictions
}

func TestJapaneseSpeechRhythmUsesInternalTimingCapability(t *testing.T) {
	morae := []frontend.Mora{
		{Text: "か", Consonant: "k", Vowel: "a"},
		{Text: "き", Consonant: "k", Vowel: "i"},
	}
	want := japaneseSpeechRhythmPredictions(morae, japaneseSpeechTestPredictions(len(morae)))

	// capabilityありはSpeechTiming無効でも常時適用する。
	internal := Config{RendererCapabilities: &plugin.Capabilities{InternalTiming: true}}
	if got := applyJapaneseSpeechRhythm(internal, nil, morae, japaneseSpeechTestPredictions(len(morae)), nil); !reflect.DeepEqual(got, want) {
		t.Fatalf("internal timing capability did not apply rhythm: %#v", got)
	}
	// capabilityなしはSpeechTimingゲートのまま。
	gated := Config{}
	if got := applyJapaneseSpeechRhythm(gated, nil, morae, japaneseSpeechTestPredictions(len(morae)), nil); !reflect.DeepEqual(got, japaneseSpeechTestPredictions(len(morae))) {
		t.Fatalf("rhythm applied without SpeechTiming: %#v", got)
	}
	// SpeechTiming有効なら通常どおり適用する。
	timed := Config{SpeechTiming: true}
	if got := applyJapaneseSpeechRhythm(timed, nil, morae, japaneseSpeechTestPredictions(len(morae)), nil); !reflect.DeepEqual(got, want) {
		t.Fatalf("SpeechTiming did not apply rhythm: %#v", got)
	}
	// ProsodyPitchOnlyは恒等。
	pitchOnly := Config{SpeechTiming: true, ProsodyPitchOnly: true, RendererCapabilities: &plugin.Capabilities{InternalTiming: true}}
	if got := applyJapaneseSpeechRhythm(pitchOnly, nil, morae, japaneseSpeechTestPredictions(len(morae)), nil); !reflect.DeepEqual(got, japaneseSpeechTestPredictions(len(morae))) {
		t.Fatalf("ProsodyPitchOnly was not identity: %#v", got)
	}
	// モデルにduration headがあれば恒等。
	model := &prosody.Model{DurationWeights: map[string]float64{"a": 1}}
	if got := applyJapaneseSpeechRhythm(internal, model, morae, japaneseSpeechTestPredictions(len(morae)), nil); !reflect.DeepEqual(got, japaneseSpeechTestPredictions(len(morae))) {
		t.Fatalf("duration head model was not identity: %#v", got)
	}
}

func TestSpeechExperimentRequiresRendererCapability(t *testing.T) {
	cfg := Config{Language: "en", Phonemizer: "en-delta", Renderer: "custom", SpeechProsodyExperiment: "pitch"}
	if validateSpeechExperiment(cfg) == nil {
		t.Fatal("experiment accepted without renderer capability")
	}
	cfg.RendererCapabilities = &plugin.Capabilities{SpeechProsodyExperiment: true}
	if err := validateSpeechExperiment(cfg); err != nil {
		t.Fatalf("experiment rejected with renderer capability: %v", err)
	}
}
