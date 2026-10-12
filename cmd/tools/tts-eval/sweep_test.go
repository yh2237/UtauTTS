package main

import "testing"

func TestDefaultSweepPresetsAreSupported(t *testing.T) {
	presets, err := resolvePresets(defaultPresets)
	if err != nil || len(presets) == 0 {
		t.Fatalf("default sweep presets = %v, error=%v", presets, err)
	}
}
