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

	internal := Config{RendererCapabilities: &plugin.Capabilities{InternalTiming: true}}
	if got := applyJapaneseSpeechRhythm(internal, nil, morae, japaneseSpeechTestPredictions(len(morae)), nil); !reflect.DeepEqual(got, want) {
		t.Fatalf("internal timing capability did not apply rhythm: %#v", got)
	}
	gated := Config{}
	if got := applyJapaneseSpeechRhythm(gated, nil, morae, japaneseSpeechTestPredictions(len(morae)), nil); !reflect.DeepEqual(got, japaneseSpeechTestPredictions(len(morae))) {
		t.Fatalf("rhythm applied without internal timing: %#v", got)
	}
	pitchOnly := Config{ProsodyPitchOnly: true, RendererCapabilities: &plugin.Capabilities{InternalTiming: true}}
	if got := applyJapaneseSpeechRhythm(pitchOnly, nil, morae, japaneseSpeechTestPredictions(len(morae)), nil); !reflect.DeepEqual(got, japaneseSpeechTestPredictions(len(morae))) {
		t.Fatalf("ProsodyPitchOnly was not identity: %#v", got)
	}
	model := &prosody.Model{DurationWeights: map[string]float64{"a": 1}}
	if got := applyJapaneseSpeechRhythm(internal, model, morae, japaneseSpeechTestPredictions(len(morae)), nil); !reflect.DeepEqual(got, japaneseSpeechTestPredictions(len(morae))) {
		t.Fatalf("duration head model was not identity: %#v", got)
	}
}
