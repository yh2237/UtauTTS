package entryinput

import (
	"encoding/json"
	"testing"
)

// TestMethodReachesInternalRequestは、合成の方法がJSONから内部の要求まで落ちずに届くことを確かめる。
func TestMethodReachesInternalRequest(t *testing.T) {
	for _, method := range []string{"native", "reference"} {
		raw := `{"text":"hello","language":"en","method":"` + method + `"}`
		var request JSONRequest
		if err := json.Unmarshal([]byte(raw), &request); err != nil {
			t.Fatal(err)
		}
		if got := request.InternalRequest().Method; got != method {
			t.Errorf("method %q became %q", method, got)
		}
	}
	// 指定がなければ空のまま（言語ごとの既定を使う）。
	var empty JSONRequest
	if err := json.Unmarshal([]byte(`{"text":"hello"}`), &empty); err != nil {
		t.Fatal(err)
	}
	if got := empty.InternalRequest().Method; got != "" {
		t.Errorf("unspecified method became %q", got)
	}
}
