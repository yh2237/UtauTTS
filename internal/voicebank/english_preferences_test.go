package voicebank

import (
	"testing"

	"utautts/internal/frontend"
)

func TestEnglishPreferencesFavorFullStopOnsetsAndCVVCTransitions(t *testing.T) {
	mora := frontend.Mora{
		Language:  frontend.LanguageEnglish,
		WordIndex: 1,
		Consonant: "p",
		Phones:    []frontend.Phone{{Symbol: "p", Role: "onset"}, {Symbol: "ae", Role: "nucleus"}},
	}
	previous := []Selection{{Mora: frontend.Mora{Language: frontend.LanguageEnglish, WordIndex: 0}}}
	candidates := []Selection{
		{Mora: mora, Alias: "ae", Kind: AliasCV},
		{Mora: mora, Alias: "p ae", Kind: AliasCV},
		{Mora: mora, Alias: "p ae", Kind: AliasCV, Composite: true, Transition: &Selection{}},
	}
	applyEnglishCandidatePreferences(candidates, previous)
	if candidates[1].PreferenceScore <= candidates[0].PreferenceScore {
		t.Fatalf("full onset was not preferred: %#v", candidates)
	}
	if candidates[2].PreferenceScore <= candidates[1].PreferenceScore {
		t.Fatalf("cross-word CVVC was not preferred: %#v", candidates)
	}
}

func TestEnglishEndingReleasePreferenceOnlyAppliesToTerminalStop(t *testing.T) {
	mora := frontend.Mora{
		Language: frontend.LanguageEnglish,
		Phones:   []frontend.Phone{{Symbol: "ao", Role: "nucleus"}, {Symbol: "d", Role: "coda"}},
	}
	released := Selection{Mora: mora, Alias: "ao d-B3"}
	unreleased := Selection{Mora: mora, Alias: "ao d"}
	if englishEndingReleasePreference(released) <= englishEndingReleasePreference(unreleased) {
		t.Fatal("release-marked stop was not preferred")
	}
	mora.Phones[1].Symbol = "l"
	if englishEndingReleasePreference(Selection{Mora: mora, Alias: "ao l-"}) != 0 {
		t.Fatal("non-stop coda received a release preference")
	}
}
