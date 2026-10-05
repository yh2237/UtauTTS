package main

import (
	"testing"
)

func TestRejectInvalidPhones(t *testing.T) {
	if e := checkedPhones([]any{map[string]any{"symbol": "a", "start_ms": 8.0, "end_ms": 7.0}}, 100); e == nil {
		t.Fatal("invalid interval accepted")
	}
}
