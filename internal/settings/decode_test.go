package settings

import (
	"encoding/json"
	"testing"
)

func TestDecodeChecksTypeAndRange(t *testing.T) {
	mora, _ := Lookup("mora_duration_ms")
	if value, err := mora.Decode(json.RawMessage(`130`)); err != nil || value != 130.0 {
		t.Fatalf("mora 130 = %v, %v", value, err)
	}
	for _, raw := range []string{`true`, `"130"`, `130.5`, `10`, `100000`} {
		if _, err := mora.Decode(json.RawMessage(raw)); err == nil {
			t.Errorf("mora_duration_ms accepted %s", raw)
		}
	}
	warp, _ := Lookup("timing_warp")
	if value, err := warp.Decode(json.RawMessage(`false`)); err != nil || value != false {
		t.Fatalf("timing_warp false = %v, %v", value, err)
	}
	if _, err := warp.Decode(json.RawMessage(`"false"`)); err == nil {
		t.Error("timing_warp accepted a string")
	}
	choice := Setting{ID: "choice", Type: "enum", Options: []Option{{Value: "a"}}}
	if _, err := choice.Decode(json.RawMessage(`"b"`)); err == nil {
		t.Error("enum accepted an unknown option")
	}
}
