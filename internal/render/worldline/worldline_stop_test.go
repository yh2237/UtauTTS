package worldline

import (
	"testing"

	"utautts/internal/frontend"
	"utautts/internal/plan"
	"utautts/internal/render/base"
	"utautts/internal/voicebank"
)

func TestE2BLegacyStopPreserveOnlyForReliableJapanese(t *testing.T) {
	p := &plan.Plan{
		Language:   "ja",
		Phonemizer: "ja-kana",
		Morae:      []frontend.Mora{{Consonant: "k", Vowel: "a"}},
	}
	unit := plan.Unit{Position: 0, Role: "mora", AliasKind: "VCV", SpeechProfile: &voicebank.SpeechProfile{TransientMS: 70, TransientConfidence: .8}}
	on, off := true, false
	if !e2bLegacyStopPreserve(p, unit, true, base.WorldlineProviderOptions{E2B: &on}) {
		t.Fatal("E2b should add a preserve-only burst in legacy Japanese mix")
	}
	if e2bLegacyStopPreserve(p, unit, false, base.WorldlineProviderOptions{E2B: &on}) {
		t.Fatal("preserve-only burst is limited to legacy mix")
	}
	unit.SpeechProfile.TransientConfidence = .6
	if e2bLegacyStopPreserve(p, unit, true, base.WorldlineProviderOptions{E2B: &on}) {
		t.Fatal("unreliable transient must not be preserved")
	}
	unit.SpeechProfile.TransientConfidence = .8
	if e2bLegacyStopPreserve(p, unit, true, base.WorldlineProviderOptions{E2B: &off}) {
		t.Fatal("E2b off must not add legacy burst")
	}
}

func TestWorldlineStopProtectionProtectsEnglishCodaPlosive(t *testing.T) {
	p := &plan.Plan{
		Language:   "en",
		Phonemizer: "en-delta",
		Morae:      []frontend.Mora{{Consonant: "m", Vowel: "iy"}},
	}
	profile := &voicebank.SpeechProfile{ReleaseTransientMS: 12, ReleaseTransientConfidence: .27}
	if !worldlineStopProtection(p, plan.Unit{Position: 0, Role: "ending", CodaPhones: []string{"t"}, SpeechProfile: profile}, base.WorldlineProviderOptions{}) {
		t.Fatal("word-final plosive must be protected")
	}
	if worldlineStopProtection(p, plan.Unit{Position: 0, Role: "ending", CodaPhones: []string{"s"}, SpeechProfile: profile}, base.WorldlineProviderOptions{}) {
		t.Fatal("fricative coda must not be protected")
	}
	silent := &voicebank.SpeechProfile{}
	if worldlineStopProtection(p, plan.Unit{Position: 0, Role: "ending", CodaPhones: []string{"t"}, SpeechProfile: silent}, base.WorldlineProviderOptions{}) {
		t.Fatal("unmeasured release must not be protected")
	}
}

func TestSpeechSourceOnsetUsesReliableNearbyLandmark(t *testing.T) {
	unit := plan.Unit{PreutteranceMS: 60, SpeechProfile: &voicebank.SpeechProfile{VoicingStartMS: 68, VoicingConfidence: .9, TransitionConfidence: .8}}
	if got := speechSourceOnsetMS(unit); got != 68 {
		t.Fatalf("source onset=%v", got)
	}
	unit.SpeechProfile.VoicingStartMS = 100
	if got := speechSourceOnsetMS(unit); got != 60 {
		t.Fatalf("distant source onset=%v", got)
	}
	unit.SpeechProfile.VoicingStartMS = 65
	unit.SpeechProfile.TransitionConfidence = .2
	if got := speechSourceOnsetMS(unit); got != 60 {
		t.Fatalf("uncertain source onset=%v", got)
	}
}

func TestWorldlineGapRepairOnlyAcceptsRepeatedVowelWithoutOnset(t *testing.T) {
	p := &plan.Plan{
		Morae: []frontend.Mora{{Vowel: "a"}, {Vowel: "a"}, {Consonant: "k", Vowel: "a"}},
		Units: []plan.Unit{
			{Position: 0, Role: "mora"},
			{Position: 1, Role: "mora"},
			{Position: 2, Role: "mora"},
		},
	}
	if !worldlineGapRepairEligible(p, 1) {
		t.Fatal("repeated vowel boundary should allow gap repair")
	}
	if worldlineGapRepairEligible(p, 2) {
		t.Fatal("consonant onset boundary must preserve its unvoiced interval")
	}
}

func TestWorldlineStopProtectionMatrix(t *testing.T) {
	on, off := true, false
	ja := &plan.Plan{Language: "ja", Phonemizer: "ja-kana", Morae: []frontend.Mora{{Consonant: "k", Vowel: "a"}}}
	en := &plan.Plan{Language: "en", Phonemizer: "en-delta", Morae: []frontend.Mora{{Consonant: "k", Vowel: "ae"}}}
	reliable := &voicebank.SpeechProfile{TransientMS: 70, TransientConfidence: .8}
	cases := []struct {
		name    string
		plan    *plan.Plan
		unit    plan.Unit
		options base.WorldlineProviderOptions
		want    bool
	}{
		{"Japanese VCV without measurement has no burst", ja, plan.Unit{Role: "mora", AliasKind: "VCV"}, base.WorldlineProviderOptions{}, false},
		{"English VCV keeps plosive protection", en, plan.Unit{Role: "mora", AliasKind: "VCV", SpeechProfile: reliable}, base.WorldlineProviderOptions{}, true},
		{"Japanese CV keeps protection", ja, plan.Unit{Role: "mora", AliasKind: "CV", SpeechProfile: reliable}, base.WorldlineProviderOptions{}, true},
		{"uncertain transient is skipped", ja, plan.Unit{Role: "mora", AliasKind: "CV", SpeechProfile: &voicebank.SpeechProfile{TransientMS: 70, TransientConfidence: .2}}, base.WorldlineProviderOptions{}, false},
		{"E2b protects reliable VCV", ja, plan.Unit{Role: "mora", AliasKind: "VCV", SpeechProfile: reliable}, base.WorldlineProviderOptions{E2B: &on}, true},
		{"E2b gates low confidence", ja, plan.Unit{Role: "mora", AliasKind: "VCV", SpeechProfile: &voicebank.SpeechProfile{TransientMS: 70, TransientConfidence: .6}}, base.WorldlineProviderOptions{E2B: &on}, false},
		{"E2b off disables VCV", ja, plan.Unit{Role: "mora", AliasKind: "VCV", SpeechProfile: reliable}, base.WorldlineProviderOptions{E2B: &off}, false},
		{"E2b off disables CV", ja, plan.Unit{Role: "mora", AliasKind: "CV", SpeechProfile: reliable}, base.WorldlineProviderOptions{E2B: &off}, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := worldlineStopProtection(test.plan, test.unit, test.options); got != test.want {
				t.Fatalf("stop protection = %t, want %t", got, test.want)
			}
		})
	}
}
