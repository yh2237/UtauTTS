package render

import (
	"testing"
	"utautts/internal/frontend"
	"utautts/internal/plan"
)

func TestCodaReleaseScope(t *testing.T) {
	p := &plan.Plan{Phonemizer: frontend.PhonemizerEnglishDelta, Morae: []frontend.Mora{{Phones: []frontend.Phone{{Symbol: "p", Role: "coda"}}}}}
	u := plan.Unit{Position: 0, Role: "ending", Alias: "V p-", CodaPhones: []string{"p"}}
	if !worldCodaReleaseEligible(p, u) {
		t.Fatal("verified p not enabled by default")
	}
	for _, alias := range []string{"V t-", "V k-", "V p t", "V p-B3", "pV"} {
		v := u
		v.Alias = alias
		if !worldCodaReleaseEligible(p, v) {
			t.Fatal("required coda rejected because of alias", alias)
		}
	}
	v := u
	v.Role = "transition"
	if worldCodaReleaseEligible(p, v) {
		t.Fatal("transition changed")
	}
	p.Phonemizer = frontend.PhonemizerEnglishVCCV
	if !worldCodaReleaseEligible(p, u) {
		t.Fatal("VCCV coda skipped")
	}
	p.Phonemizer = frontend.PhonemizerEnglishDelta
	p.Morae[0].Phones[0].Role = "onset"
	u.CodaPhones = nil
	if worldCodaReleaseEligible(p, u) {
		t.Fatal("optional ending mistaken for required coda")
	}
	if !codaReleaseStop(plan.Unit{CodaPhones: []string{"d"}}) || codaReleaseStop(plan.Unit{CodaPhones: []string{"l"}}) {
		t.Fatal("wrong stop classification")
	}
}

func TestCodaReleaseEnvelopeKeepsConsonantAudible(t *testing.T) {
	u := plan.Unit{DurationMS: 30, CodaPhones: []string{"d"}}
	points := []worldlineEnvelopePoint{{XMS: -30}, {XMS: -20}, {XMS: 0}, {XMS: 0}, {XMS: 30}}
	got, fade := codaReleaseEnvelope(u, points, 30)
	if fade != 5 || got[3].XMS != 25 || got[4].XMS != 30 {
		t.Fatalf("fade=%v points=%#v", fade, got)
	}
}
