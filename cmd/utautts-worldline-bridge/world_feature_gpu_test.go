package main

import "testing"

func TestCUDARejectsMalformedFeaturesBeforeNativeCall(t *testing.T) {
	input, prepared, fftSize := worldMixFixture()
	prepared[0].cached.features.Spectrum = nil
	if _, err := mixWorldFeaturesCUDA("must-not-load", input, prepared, fftSize); err == nil {
		t.Fatal("malformed source accepted")
	}
}
