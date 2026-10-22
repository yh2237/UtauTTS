package tts

import "testing"

// TestValidateMethodは、方式の指定を確かめる。
func TestValidateMethod(t *testing.T) {
	for _, method := range []string{MethodWaveformSynthesis, MethodReferenceMatching} {
		if err := ValidateMethod(method); err != nil {
			t.Fatalf("ValidateMethod(%q) = %v", method, err)
		}
	}
	for _, method := range []string{"", "native", "reference", "waveform"} {
		if err := ValidateMethod(method); err == nil {
			t.Fatalf("ValidateMethod(%q) accepted an unsupported method", method)
		}
	}
}
