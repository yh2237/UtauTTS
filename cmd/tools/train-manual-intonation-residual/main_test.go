package main

import (
	"testing"
)

func TestTimingFallback(t *testing.T) {
	start, duration := timing(map[string]any{}, []string{"あ", "", "い"}, []bool{false, true, false})
	if len(start) != 3 || start[0] != 0 || start[1] != 120 || start[2] != 300 || duration[1] != 180 {
		t.Fatalf("fallback timing: %v %v", start, duration)
	}
}
