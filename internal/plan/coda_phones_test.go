package plan

import (
	"reflect"
	"testing"
	"utautts/internal/frontend"
	"utautts/internal/voicebank"
)

func TestCodaMetadataUsesEndingGroupAndClones(t *testing.T) {
	m := frontend.Mora{Language: "en", Text: "test", Phones: []frontend.Phone{{Symbol: "eh", Role: "nucleus"}, {Symbol: "s", Role: "coda"}, {Symbol: "t", Role: "coda"}}, Aliases: &frontend.AliasHints{EndingPhones: [][]string{{"s"}, {"t"}, {}}}}
	selections := []voicebank.Selection{{Position: 0, Mora: m, Endings: []voicebank.Selection{{EndingIndex: 0}, {EndingIndex: 1}, {EndingIndex: 2}}}}
	p, err := Build(&voicebank.Bank{}, "test", []frontend.Mora{m}, selections, Config{MoraDurationMS: 200})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Units[1].CodaPhones, []string{"s"}) || !reflect.DeepEqual(p.Units[2].CodaPhones, []string{"t"}) || len(p.Units[3].CodaPhones) != 0 {
		t.Fatal("wrong ending group", p.Units)
	}
	copy := Clone(p)
	copy.Units[1].CodaPhones[0] = "x"
	if p.Units[1].CodaPhones[0] != "s" || m.Aliases.EndingPhones[0][0] != "s" {
		t.Fatal("coda metadata shared")
	}
}
