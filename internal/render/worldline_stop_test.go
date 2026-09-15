package render

import (
	"testing"

	"utautts/internal/frontend"
	"utautts/internal/plan"
	"utautts/internal/voicebank"
)

func TestWorldlineStopProtectionSkipsJapaneseVCV(t *testing.T) {
	p := &plan.Plan{
		Language:   "ja",
		Phonemizer: "ja-kana",
		Morae:      []frontend.Mora{{Consonant: "k", Vowel: "a"}},
	}
	if worldlineStopProtection(p, plan.Unit{Position: 0, Role: "mora", AliasKind: "VCV"}) {
		t.Fatal("Japanese VCV must not receive a raw stop burst")
	}
}

func TestWorldlineStopProtectionKeepsEnglishPlosiveSupport(t *testing.T) {
	p := &plan.Plan{
		Language:   "en",
		Phonemizer: "en-delta",
		Morae:      []frontend.Mora{{Consonant: "k", Vowel: "ae"}},
	}
	profile := &voicebank.SpeechProfile{TransientMS: 70, TransientConfidence: .8}
	if !worldlineStopProtection(p, plan.Unit{Position: 0, Role: "mora", AliasKind: "VCV", SpeechProfile: profile}) {
		t.Fatal("English VCV plosive must remain protected")
	}
}

func TestWorldlineStopProtectionKeepsJapaneseCVSupport(t *testing.T) {
	p := &plan.Plan{
		Language:   "ja",
		Phonemizer: "ja-kana",
		Morae:      []frontend.Mora{{Consonant: "k", Vowel: "a"}},
	}
	profile := &voicebank.SpeechProfile{TransientMS: 70, TransientConfidence: .8}
	if !worldlineStopProtection(p, plan.Unit{Position: 0, Role: "mora", AliasKind: "CV", SpeechProfile: profile}) {
		t.Fatal("Japanese CV onset should keep the short stop protection")
	}
	profile.TransientConfidence = .2
	if worldlineStopProtection(p, plan.Unit{Position: 0, Role: "mora", AliasKind: "CV", SpeechProfile: profile}) {
		t.Fatal("uncertain transient must not be mixed")
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
