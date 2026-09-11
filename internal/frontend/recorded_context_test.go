package frontend

import (
	"reflect"
	"strings"
	"testing"
)

func TestRecordedContextPreservesPhonesAndFallback(t *testing.T) {
	for _, ph := range []string{PhonemizerEnglishDelta, PhonemizerEnglishVCCV} {
		parse := ParseEnglishDelta
		if ph == PhonemizerEnglishVCCV {
			parse = ParseEnglishVCCV
		}
		_, units, err := parse("", "HH IY1 | S T R AE1", nil)
		if err != nil {
			t.Fatal(err)
		}
		before := append([]string(nil), units[1].Aliases.Main...)
		got := RecordedContextCandidates(units, ph)
		if len(got[1].Aliases.Main) <= len(before) {
			t.Fatal("no context candidates", got)
		}
		if !reflect.DeepEqual(got[1].Aliases.Main[len(got[1].Aliases.Main)-len(before):], before) {
			t.Fatal("fallback lost")
		}
		if !reflect.DeepEqual(units[1].Aliases.Main, before) {
			t.Fatal("input mutated")
		}
		for i, kind := range got[1].Aliases.MainKinds {
			if kind == "vcv" {
				_, cv, _ := strings.Cut(got[1].Aliases.Main[i], " ")
				if !strings.HasPrefix(cv, "str") {
					t.Fatal("onset truncated", cv)
				}
			}
		}
		units[0].Pause = true
		if !reflect.DeepEqual(RecordedContextCandidates(units, ph), units) {
			t.Fatal("crossed pause")
		}
		units[0].Pause = false
		units[0].Phones = append(units[0].Phones, Phone{Symbol: "T", Role: "coda"})
		if !reflect.DeepEqual(RecordedContextCandidates(units, ph), units) {
			t.Fatal("crossed coda")
		}
	}
}
