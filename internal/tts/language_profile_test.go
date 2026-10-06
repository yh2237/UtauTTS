package tts

import (
	"path/filepath"
	"testing"
)

func TestProsodyFallbackModelPath(t *testing.T) {
	configured := filepath.Join("models", "frame-intonation-tcn-v10.json")
	cases := []struct {
		language string
		want     string
	}{
		{"en-US", filepath.Join("models", "frame-intonation-tcn-en-v1.json")},
		{"zh-cn", filepath.Join("models", "tone-intonation-zh-v1.json")},
		{"ja", ""},
		{"fr", ""},
	}
	for _, test := range cases {
		if got := prosodyFallbackModelPath(configured, test.language); got != test.want {
			t.Fatalf("prosodyFallbackModelPath(%q) = %q, want %q", test.language, got, test.want)
		}
	}
}
