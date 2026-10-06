package frontend

import "testing"

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
