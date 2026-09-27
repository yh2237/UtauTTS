package frontend

import (
	"strings"
	"testing"
)

func TestEnglishReadUsesUnambiguousVerbContext(t *testing.T) {
	for _, tc := range []struct{ text, reading string }{
		{"Please read this text aloud.", "R IY1 D"},
		{"Are you ready to read the next chapter?", "R IY1 D"},
		{"Read this book.", "R IY1 D"},
		{"I will read this book.", "R IY1 D"},
		{"I did read this book.", "R IY1 D"},
		{"I have read this book.", "R EH1 D"},
		{"I read it yesterday.", "R EH1 D"},
	} {
		got, _, err := ParseEnglishDelta(tc.text, "", nil)
		if err != nil || !strings.Contains(got, tc.reading) {
			t.Fatalf("%q = %q, %v", tc.text, got, err)
		}
	}
	got, _, err := ParseEnglishVCCV("Please read this.", "", map[string]string{"read": "R EH1 D"})
	if err != nil || !strings.Contains(got, "R EH1 D") {
		t.Fatalf("dictionary override lost: %q, %v", got, err)
	}
	got, _, err = ParseEnglishDelta("Please read this.", "R EH1 D", nil)
	if err != nil || got != "R EH1 D" {
		t.Fatalf("explicit reading lost: %q, %v", got, err)
	}
}
