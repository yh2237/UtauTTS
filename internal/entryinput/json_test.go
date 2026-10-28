package entryinput

import (
	"encoding/json"
	"testing"
)

func TestJSONRequestPreservesSynthesisFields(t *testing.T) {
	const method = "waveform-synthesis"
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
	var empty JSONRequest
	if err := json.Unmarshal([]byte(`{"text":"hello"}`), &empty); err != nil {
		t.Fatal(err)
	}
	if got := empty.InternalRequest().Method; got != "" {
		t.Errorf("unspecified method became %q", got)
	}
}

func TestJSONRequestRejectsRemovedFields(t *testing.T) {
	for _, field := range []string{`"renderer":"x"`, `"diffsinger_steps":20`, `"resampler":"x.exe"`, `"speech_timing":true`} {
		var request JSONRequest
		if err := json.Unmarshal([]byte(`{"text":"あ",`+field+`}`), &request); err == nil {
			t.Errorf("%s was accepted", field)
		}
	}
}
