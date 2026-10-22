package entryinput

import (
	"encoding/json"
	"testing"
)

func TestJSONRequestPreservesSynthesisFields(t *testing.T) {
	for _, method := range []string{"waveform-synthesis", "reference-matching"} {
		raw := `{"text":"hello","language":"en","voicebank_id":"bank","method":"` + method + `","apply_pitch":false,"intonation_strength":0}`
		var request JSONRequest
		if err := json.Unmarshal([]byte(raw), &request); err != nil {
			t.Fatal(err)
		}
		input := request.InternalRequest()
		if input.Method != method || input.Language != "en" || input.VoicebankID != "bank" || input.Text != "hello" {
			t.Errorf("shared request fields were lost: %+v", input)
		}
		if input.ApplyPitch || input.IntonationStrength != 0 {
			t.Fatalf("explicit settings were replaced by defaults: %+v", input)
		}
	}
	var empty JSONRequest
	if err := json.Unmarshal([]byte(`{"text":"hello"}`), &empty); err != nil {
		t.Fatal(err)
	}
	if got := empty.InternalRequest().Method; got != "" {
		t.Errorf("unspecified method became %q", got)
	}
}

func TestJSONRequestAcceptsRemovedClassicFields(t *testing.T) {
	raw := `{"text":"あ","resampler":"x.exe","wavtool":"builtin","resampler_expressions":[{"position":0,"velocity":90}],` +
		`"unit_overrides":[{"unit_index":0,"resampler_volume":110,"resampler_velocity":90,"resampler_flags":"g-3","resampler_modulation":20,"resampler_tempo":135}]}`
	var request JSONRequest
	if err := json.Unmarshal([]byte(raw), &request); err != nil {
		t.Fatal(err)
	}
	input := request.InternalRequest()
	if len(input.UnitOverrides) != 1 || input.UnitOverrides[0].ResamplerVolume == nil || *input.UnitOverrides[0].ResamplerVolume != 110 {
		t.Fatalf("unit overrides = %+v", input.UnitOverrides)
	}
}
