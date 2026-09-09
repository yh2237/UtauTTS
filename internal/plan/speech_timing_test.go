package plan

import (
	"math"
	"testing"
	"utautts/internal/frontend"
	"utautts/internal/oto"
	"utautts/internal/voicebank"
)

func TestSpeechCodaUsesPhoneBudgetAndRetainsGaps(t *testing.T) {
	_, morae, err := frontend.ParseEnglishDelta("", "T EH1 K S T", nil)
	if err != nil {
		t.Fatal(err)
	}
	selected := []voicebank.Selection{{Position: 0, Mora: morae[0], Alias: "tE", Entry: oto.Entry{Filename: "main.wav"},
		Endings:       []voicebank.Selection{{Alias: "k st-", EndingIndex: 1, Entry: oto.Entry{Filename: "ending.wav"}}},
		MissingPhones: []voicebank.SpeechGap{{Position: 0, Role: "coda", Phones: []string{"k"}}},
	}}
	p, err := Build(&voicebank.Bank{}, "", morae, selected, Config{MoraDurationMS: 120, MoraDurationsMS: []float64{180}})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.PhoneTimings) != 5 || p.DurationMS != 180 || len(p.MissingPhones) != 1 {
		t.Fatalf("plan: %+v", p)
	}
	total := 0.0
	for _, phone := range p.PhoneTimings {
		total += phone.DurationMS
	}
	if math.Abs(total-180) > 1e-8 {
		t.Fatal("phone budget changed manual duration")
	}
	end := p.Units[1]
	if math.Abs(end.NoteStartMS-p.PhoneTimings[3].StartMS) > 1e-8 || math.Abs(end.NoteStartMS+end.DurationMS-180) > 1e-8 {
		t.Fatalf("lost coda slot after missing k: %+v", end)
	}
	clone := Clone(p)
	clone.MissingPhones[0].Phones[0] = "changed"
	clone.Morae[0].Phones[0].Symbol = "changed"
	if p.MissingPhones[0].Phones[0] != "k" || p.Morae[0].Phones[0].Symbol != "t" {
		t.Fatal("clone aliases speech metadata")
	}
}
