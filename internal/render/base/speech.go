package base

import (
	"strings"

	"utautts/internal/plan"
)

func SpeechStop(p *plan.Plan, unit plan.Unit) bool {
	// 語末の破裂音は親モーラのonsetとは独立に保護対象にする。
	if CodaReleaseStop(unit) {
		return true
	}
	if unit.Position < 0 || unit.Position >= len(p.Morae) {
		return false
	}
	if isStopPhone(p.Morae[unit.Position].Consonant) {
		return true
	}
	for _, phone := range p.Morae[unit.Position].Phones {
		if phone.Role == "onset" && isStopPhone(phone.Symbol) {
			return true
		}
	}
	return false
}

func isStopPhone(phone string) bool {
	return strings.Contains(" p py b by t d k ky g gy ", " "+strings.ToLower(strings.TrimSpace(phone))+" ")
}

// SpeechVowelJoinは自動補正が同母音の連続だけに限られるかを返す。
func SpeechVowelJoin(p *plan.Plan, previous, current RenderedUnit) bool {
	if previous.Index+1 != current.Index || previous.Unit.Role != "mora" || current.Unit.Role != "mora" || previous.Unit.Position+1 != current.Unit.Position {
		return false
	}
	if previous.Unit.Position < 0 || current.Unit.Position >= len(p.Morae) {
		return false
	}
	a, b := p.Morae[previous.Unit.Position], p.Morae[current.Unit.Position]
	if a.Pause || b.Pause || a.Vowel == "" || a.Vowel != b.Vowel || a.Vowel == "cl" || a.Vowel == "n" || b.Consonant != "" {
		return false
	}
	for _, phone := range a.Phones {
		if phone.Role == "coda" {
			return false
		}
	}
	for _, phone := range b.Phones {
		if phone.Role == "onset" {
			return false
		}
	}
	for _, unit := range []plan.Unit{previous.Unit, current.Unit} {
		if unit.SpeechProfile == nil || !unit.SpeechProfile.Applied {
			return false
		}
	}
	return true
}

func CodaReleaseStop(u plan.Unit) bool {
	for _, phone := range u.CodaPhones {
		if strings.Contains(" p b t d k g ch jh ", " "+strings.ToLower(phone)+" ") {
			return true
		}
	}
	return false
}
