package main

import (
	"testing"
)

func TestRejectGeneratedObservation(t *testing.T) {
	template := map[string]any{"version": float64(1), "feature_version": float64(1), "id": "x"}
	obs := map[string]any{"id": "x", "speaker": "s", "corpus": "c", "license": "l", "audio_path": "x.wav", "kind": "generated", "alignment": "manual", "split": "train"}
	if _, e := prepare(template, obs, "out"); e == nil {
		t.Fatal("generated observation accepted")
	}
}
