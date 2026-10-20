package tts

import "testing"

// TestUsesReferenceSynthesisは、方式の指定だけで参照照合を選ぶことを確かめる。
func TestUsesReferenceSynthesis(t *testing.T) {
	tests := []struct {
		language, method string
		want             bool
	}{
		{"en", MethodWaveformSynthesis, false},
		{"ja", MethodWaveformSynthesis, false},
		{"en", MethodReferenceMatching, true},
		{"ja", MethodReferenceMatching, true},
		// 方式は必須なので、空や不明な値で参照照合に倒れない。
		{"en", "", false},
		{"ja", "", false},
	}
	for _, test := range tests {
		if got := usesReferenceSynthesis(test.language, test.method); got != test.want {
			t.Fatalf("usesReferenceSynthesis(%q, %q) = %v, want %v", test.language, test.method, got, test.want)
		}
	}
}

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
