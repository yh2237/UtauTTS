package render

import (
	"math"
	"testing"

	"utautts/internal/frontend"
	"utautts/internal/plan"
	"utautts/internal/voicebank"
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

func TestCodaClosureReleaseSplitSeparatesEnglishStop(t *testing.T) {
	u := plan.Unit{Role: "ending", CodaPhones: []string{"t"}, DurationMS: 70,
		SpeechProfile: &voicebank.SpeechProfile{ReleaseTransientMS: 40, ReleaseTransientDurationMS: 16, ReleaseTransientConfidence: .6}}
	closure, release, ok := codaClosureReleaseSplit(u)
	if !ok {
		t.Fatal("english stop coda not split")
	}
	if math.Abs(closure+release-u.DurationMS) > 1e-9 {
		t.Fatalf("total length changed: closure=%v release=%v", closure, release)
	}
	if release < codaReleaseMinMS || release > codaReleaseMaxMS || closure < codaClosureMinMS {
		t.Fatalf("unbounded split: closure=%v release=%v", closure, release)
	}
	// 破裂音でないcodaは対象外。
	if _, _, ok := codaClosureReleaseSplit(plan.Unit{CodaPhones: []string{"s"}, DurationMS: 70}); ok {
		t.Fatal("fricative coda must not split")
	}
	// 短すぎるcodaは対象外。
	if _, _, ok := codaClosureReleaseSplit(plan.Unit{CodaPhones: []string{"t"}, DurationMS: 20}); ok {
		t.Fatal("short coda must not split")
	}
}

func TestE2ACodaReleaseSplitRespectsToggle(t *testing.T) {
	defer SetE2A(true)
	p := &plan.Plan{Phonemizer: frontend.PhonemizerEnglishDelta, Morae: []frontend.Mora{{Consonant: "m", Vowel: "iy"}}}
	u := plan.Unit{Position: 0, Role: "ending", CodaPhones: []string{"t"}, DurationMS: 70,
		SpeechProfile: &voicebank.SpeechProfile{ReleaseTransientMS: 40, ReleaseTransientDurationMS: 16, ReleaseTransientConfidence: .6}}
	SetE2A(true)
	if _, _, ok := worldCodaReleaseSplit(p, u); !ok {
		t.Fatal("E2a on should split english stop coda")
	}
	SetE2A(false)
	if _, _, ok := worldCodaReleaseSplit(p, u); ok {
		t.Fatal("E2a off must not split")
	}
	// 日本語はCodaPhonesが付かないため対象外。
	ja := &plan.Plan{Language: "ja", Phonemizer: "ja-kana"}
	SetE2A(true)
	if _, _, ok := worldCodaReleaseSplit(ja, u); ok {
		t.Fatal("non-English coda must not split")
	}
}
