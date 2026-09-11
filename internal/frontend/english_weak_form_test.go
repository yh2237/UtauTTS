package frontend

import (
	"strings"
	"testing"
)

func TestOfWeakFormOnlyAutomaticPhraseInterior(t *testing.T) {
	for _, tc := range []struct {
		text, reading string
		dict          map[string]string
		weak          bool
	}{
		{text: "Another cup of coffee.", weak: true},
		{text: "A cup OF coffee.", weak: true},
		{text: "of"}, {text: "Of course."}, {text: "What is it made of?"},
		{text: "cup, of coffee"}, {text: "cup of, coffee"},
		{text: "cup of coffee", reading: "K AH1 P | AH1 V | K AA1 F IY0"},
		{text: "cup of coffee", dict: map[string]string{"of": "AH1 V"}},
	} {
		got, _, err := englishPronunciation(tc.text, tc.reading, tc.dict)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(got, "AH0 V") != tc.weak {
			t.Fatalf("%+v: %s", tc, got)
		}
	}
}

func TestAutomaticOfMatchesAcceptedManualReading(t *testing.T) {
	for _, parse := range []func(string, string, map[string]string) (string, []Mora, error){ParseEnglishDelta, ParseEnglishVCCV} {
		got, _, err := parse("Another cup of coffee.", "", nil)
		if err != nil {
			t.Fatal(err)
		}
		want := "AH0 N AH1 DH ER0 | K AH1 P | AH0 V | K AA1 F IY0 | SP"
		if got != want {
			t.Fatalf("%s != %s", got, want)
		}
	}
}
