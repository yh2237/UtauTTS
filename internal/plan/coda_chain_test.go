package plan

import (
	"math"
	"testing"
	"utautts/internal/frontend"
	"utautts/internal/voicebank"
)

func TestExpandedCodaUnitsKeepIndividualPhoneSlots(t *testing.T) {
	_, m, _ := frontend.ParseEnglishDelta("", "T EH1 K S T S", nil)
	endings := []voicebank.Selection{}
	for i, p := range []string{"k", "s", "t", "s"} {
		endings = append(endings, voicebank.Selection{EndingIndex: i, CodaStart: i, CodaPhones: []string{p}})
	}
	p, err := Build(&voicebank.Bank{}, "", m, []voicebank.Selection{{Mora: m[0], Endings: endings}}, Config{MoraDurationMS: 400, PhoneWeights: [][]float64{{1, 2, 1, 1, 1, 1}}, PhoneWeightsSource: "multilingual-speech-score-v1"})
	if err != nil {
		t.Fatal(err)
	}
	for i, u := range p.Units[1:] {
		phone := p.PhoneTimings[i+2]
		if math.Abs(u.NoteStartMS-phone.StartMS) > .001 || math.Abs(u.DurationMS-phone.DurationMS) > .001 || u.CodaPhones[0] != phone.Symbol {
			t.Fatal(i, u, phone)
		}
	}
	copy := Clone(p)
	copy.Morae[0].Aliases.EndingFallbacks[1][0].Phones[0] = "x"
	if p.Morae[0].Aliases.EndingFallbacks[1][0].Phones[0] != "s" {
		t.Fatal("fallback hints shared")
	}
}
