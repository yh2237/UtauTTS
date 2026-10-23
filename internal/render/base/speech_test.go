package base

import (
	"testing"

	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/voicebank"
)

func TestSpeechStopUsesMoraConsonantWithoutPhoneMetadata(t *testing.T) {
	p := &plan.Plan{Morae: []frontend.Mora{{Consonant: "k", Vowel: "a"}}}
	if !SpeechStop(p, plan.Unit{Position: 0}) {
		t.Fatal("stop consonant was not detected without phone metadata")
	}
	p.Morae[0].Consonant = "s"
	if SpeechStop(p, plan.Unit{Position: 0}) {
		t.Fatal("fricative was classified as a stop")
	}
}

func TestSpeechStopIncludesCodaPlosive(t *testing.T) {
	p := &plan.Plan{Morae: []frontend.Mora{{Consonant: "m", Vowel: "iy"}}}
	if !SpeechStop(p, plan.Unit{Position: 0, Role: "ending", CodaPhones: []string{"t"}}) {
		t.Fatal("coda plosive was not detected")
	}
	if SpeechStop(p, plan.Unit{Position: 0, Role: "ending", CodaPhones: []string{"s"}}) {
		t.Fatal("coda fricative was classified as a stop")
	}
}

func TestSpeechJoinProtectsConsonantsAndUnknownProfiles(t *testing.T) {
	p := &plan.Plan{Morae: []frontend.Mora{{Vowel: "a"}, {Vowel: "a"}}}
	left := RenderedUnit{Index: 0, Unit: plan.Unit{Role: "mora", Position: 0, SpeechProfile: &voicebank.SpeechProfile{Applied: true}}}
	right := RenderedUnit{Index: 1, Unit: plan.Unit{Role: "mora", Position: 1, SpeechProfile: &voicebank.SpeechProfile{Applied: true}}}
	if !SpeechVowelJoin(p, left, right) {
		t.Fatal("repeated vowel rejected")
	}
	p.Morae[1].Consonant = "k"
	if SpeechVowelJoin(p, left, right) {
		t.Fatal("stop accepted")
	}
	p.Morae[1].Consonant = ""
	p.Morae[0].Phones = []frontend.Phone{{Symbol: "s", Role: "coda"}}
	if SpeechVowelJoin(p, left, right) {
		t.Fatal("coda accepted")
	}
	p.Morae[0].Phones = nil
	p.Morae[1].Vowel = "i"
	if SpeechVowelJoin(p, left, right) {
		t.Fatal("different vowel accepted")
	}
	p.Morae[1].Vowel = "a"
	right.Unit.SpeechProfile.Applied = false
	if SpeechVowelJoin(p, left, right) {
		t.Fatal("uncertain profile accepted")
	}
}
