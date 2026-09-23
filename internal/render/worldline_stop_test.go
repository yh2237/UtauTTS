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
	if worldlineStopProtection(p, plan.Unit{Position: 0, Role: "mora", AliasKind: "VCV"}, WorldlineProviderOptions{}) {
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
	if !worldlineStopProtection(p, plan.Unit{Position: 0, Role: "mora", AliasKind: "VCV", SpeechProfile: profile}, WorldlineProviderOptions{}) {
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
	if !worldlineStopProtection(p, plan.Unit{Position: 0, Role: "mora", AliasKind: "CV", SpeechProfile: profile}, WorldlineProviderOptions{}) {
		t.Fatal("Japanese CV onset should keep the short stop protection")
	}
	profile.TransientConfidence = .2
	if worldlineStopProtection(p, plan.Unit{Position: 0, Role: "mora", AliasKind: "CV", SpeechProfile: profile}, WorldlineProviderOptions{}) {
		t.Fatal("uncertain transient must not be mixed")
	}
}

func TestWorldlineStopProtectionE2BGeneralizesJapanesePlosives(t *testing.T) {
	p := &plan.Plan{
		Language:   "ja",
		Phonemizer: "ja-kana",
		Morae:      []frontend.Mora{{Consonant: "k", Vowel: "a"}},
	}
	vcv := plan.Unit{Position: 0, Role: "mora", AliasKind: "VCV", SpeechProfile: &voicebank.SpeechProfile{TransientMS: 70, TransientConfidence: .8}}
	cv := plan.Unit{Position: 0, Role: "mora", AliasKind: "CV", SpeechProfile: &voicebank.SpeechProfile{TransientMS: 70, TransientConfidence: .7}}
	on, off := true, false
	if !worldlineStopProtection(p, vcv, WorldlineProviderOptions{E2B: &on}) || !worldlineStopProtection(p, cv, WorldlineProviderOptions{E2B: &on}) {
		t.Fatal("E2b on should protect reliable Japanese VCV and CV")
	}
	// 信頼度が下限未満のVCVは保護しない。
	vcv.SpeechProfile.TransientConfidence = .6
	if worldlineStopProtection(p, vcv, WorldlineProviderOptions{E2B: &on}) {
		t.Fatal("E2b must gate Japanese VCV on high confidence")
	}
	if worldlineStopProtection(p, vcv, WorldlineProviderOptions{E2B: &off}) || worldlineStopProtection(p, cv, WorldlineProviderOptions{E2B: &off}) {
		t.Fatal("E2b off must not protect Japanese plosives")
	}
	// 未指定は既定ON。
	if !worldlineStopProtection(p, cv, WorldlineProviderOptions{}) {
		t.Fatal("E2b default should be on")
	}
}

func TestE2BLegacyStopPreserveOnlyForReliableJapanese(t *testing.T) {
	p := &plan.Plan{
		Language:   "ja",
		Phonemizer: "ja-kana",
		Morae:      []frontend.Mora{{Consonant: "k", Vowel: "a"}},
	}
	unit := plan.Unit{Position: 0, Role: "mora", AliasKind: "VCV", SpeechProfile: &voicebank.SpeechProfile{TransientMS: 70, TransientConfidence: .8}}
	on, off := true, false
	if !e2bLegacyStopPreserve(p, unit, true, WorldlineProviderOptions{E2B: &on}) {
		t.Fatal("E2b should add a preserve-only burst in legacy Japanese mix")
	}
	if e2bLegacyStopPreserve(p, unit, false, WorldlineProviderOptions{E2B: &on}) {
		t.Fatal("preserve-only burst is limited to legacy mix")
	}
	unit.SpeechProfile.TransientConfidence = .6
	if e2bLegacyStopPreserve(p, unit, true, WorldlineProviderOptions{E2B: &on}) {
		t.Fatal("unreliable transient must not be preserved")
	}
	unit.SpeechProfile.TransientConfidence = .8
	if e2bLegacyStopPreserve(p, unit, true, WorldlineProviderOptions{E2B: &off}) {
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
	if !worldlineStopProtection(p, plan.Unit{Position: 0, Role: "ending", CodaPhones: []string{"t"}, SpeechProfile: profile}, WorldlineProviderOptions{}) {
		t.Fatal("word-final plosive must be protected")
	}
	// codaが破裂音でなければ対象外。
	if worldlineStopProtection(p, plan.Unit{Position: 0, Role: "ending", CodaPhones: []string{"s"}, SpeechProfile: profile}, WorldlineProviderOptions{}) {
		t.Fatal("fricative coda must not be protected")
	}
	// 解放過渡が測れなければ対象外。
	silent := &voicebank.SpeechProfile{}
	if worldlineStopProtection(p, plan.Unit{Position: 0, Role: "ending", CodaPhones: []string{"t"}, SpeechProfile: silent}, WorldlineProviderOptions{}) {
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
