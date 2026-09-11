package frontend

import (
	"reflect"
	"testing"
)

func TestCodaVowelExperimentPreservesCoda(t *testing.T) {
	_, m, err := ParseEnglishDelta("", "K AH1 P | AH0 V | SP", nil)
	if err != nil {
		t.Fatal(err)
	}
	original := append([]string(nil), m[1].Aliases.Main...)
	got := CodaVowelCandidates(m, PhonemizerEnglishDelta)
	if !containsString(got[1].Aliases.Main, "@") || containsString(got[1].Aliases.Main, "p@") {
		t.Fatal(got[1].Aliases)
	}
	if !reflect.DeepEqual(got[0], m[0]) || !reflect.DeepEqual(got[1].Phones, m[1].Phones) || !reflect.DeepEqual(m[1].Aliases.Main, original) {
		t.Fatal("coda/metadata/input changed")
	}
	m[0].Pause = true
	if !reflect.DeepEqual(CodaVowelCandidates(m, PhonemizerEnglishDelta), m) {
		t.Fatal("crossed pause")
	}
	_, m, err = ParseEnglishDelta("", "K AH1 P | K AA1 F IY0 | SP", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(CodaVowelCandidates(m, PhonemizerEnglishDelta), m) {
		t.Fatal("removed true onset")
	}
}
