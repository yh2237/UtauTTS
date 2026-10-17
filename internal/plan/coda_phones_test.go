package plan

import (
	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/voicebank"
	"reflect"
	"testing"
)

func TestMainCodaMetadata(t *testing.T) {
	for _, m := range []frontend.Mora{
		{Language: "zh", Text: "bang", Phones: []frontend.Phone{{Symbol: "b", Role: "onset"}, {Symbol: "a", Role: "nucleus"}, {Symbol: "ng", Role: "coda"}}},
	} {
		p, err := Build(&voicebank.Bank{}, "", []frontend.Mora{m}, []voicebank.Selection{{Mora: m}}, Config{MoraDurationMS: 120})
		if err != nil {
			t.Fatal(err)
		}
		want := m.Phones[len(m.Phones)-1].Symbol
		if len(p.Units) != 1 || !reflect.DeepEqual(p.Units[0].CodaPhones, []string{want}) {
			t.Fatal(p.Units)
		}
		if p.DurationMS != 120 {
			t.Fatal("metadata changed duration", p.DurationMS)
		}
	}
}
