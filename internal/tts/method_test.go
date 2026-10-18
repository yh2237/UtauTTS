package tts

import "testing"

func TestUsesReferenceSynthesis(t *testing.T) {
	tests := []struct {
		language, method string
		want             bool
	}{
		{"en", "", true},
		{"ja", "", false},
		{"zh", "", false},
		{"", "", false},
		{"en", "native", false},
		{"ja", "native", false},
		{"ja", "reference", true},
	}
	for _, test := range tests {
		if got := usesReferenceSynthesis(test.language, test.method); got != test.want {
			t.Fatalf("usesReferenceSynthesis(%q, %q) = %v, want %v", test.language, test.method, got, test.want)
		}
	}
}
