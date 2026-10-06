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

func TestNormalizeLanguage(t *testing.T) {
	cases := map[string]string{"": "ja", "JA": "ja", "ja-jp": "ja", "en-US": "en", "zh_CN": "zh", "cmn": "zh", "fr": "fr"}
	for input, want := range cases {
		if got := NormalizeLanguage(input); got != want {
			t.Fatalf("NormalizeLanguage(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestJapanesePlan(t *testing.T) {
	for _, test := range []struct {
		language, phonemizer string
		want                 bool
	}{
		{"", "", true},
		{"ja", "", true},
		{"ja-jp", "", true},
		{"en", "ja-delta", true},
		{"en", "ja", true},
		{"en", "en-delta", false},
		{"zh", "", false},
	} {
		if got := JapanesePlan(test.language, test.phonemizer); got != test.want {
			t.Fatalf("JapanesePlan(%q, %q) = %v, want %v", test.language, test.phonemizer, got, test.want)
		}
	}
}
