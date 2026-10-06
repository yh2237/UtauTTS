package base

import "testing"

func TestCorrectionAppliesTo(t *testing.T) {
	cases := []struct {
		id, language string
		want         bool
	}{
		{"microprosody", "ja", true},
		{"microprosody", "", true},
		{"microprosody", "ja-jp", true},
		{"microprosody", "en", false},
		{"timing_warp", "en", true},
		{"timing_warp", "zh-cn", true},
		{"timing_warp", "fr", true},
		{"english_weak_form", "en-us", true},
		{"english_weak_form", "ja", false},
		{"coda_release", "zh", true},
		{"coda_release", "ja", false},
		{"source_phone_library", "cmn", true},
		{"unknown", "ja", false},
	}
	for _, test := range cases {
		if got := CorrectionAppliesTo(test.id, test.language); got != test.want {
			t.Fatalf("CorrectionAppliesTo(%q, %q) = %v, want %v", test.id, test.language, got, test.want)
		}
	}
}
