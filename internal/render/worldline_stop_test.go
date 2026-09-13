package render

import (
	"testing"

	"utautts/internal/frontend"
	"utautts/internal/plan"
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
	if !worldlineStopProtection(p, plan.Unit{Position: 0, Role: "mora", AliasKind: "VCV"}) {
		t.Fatal("English VCV plosive must remain protected")
	}
}

func TestWorldlineStopProtectionKeepsJapaneseCVSupport(t *testing.T) {
	p := &plan.Plan{
		Language:   "ja",
		Phonemizer: "ja-kana",
		Morae:      []frontend.Mora{{Consonant: "k", Vowel: "a"}},
	}
	if !worldlineStopProtection(p, plan.Unit{Position: 0, Role: "mora", AliasKind: "CV"}) {
		t.Fatal("Japanese CV onset should keep the short stop protection")
	}
}
