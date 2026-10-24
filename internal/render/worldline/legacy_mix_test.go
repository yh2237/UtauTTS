package worldline

import (
	"testing"

	"github.com/yh2237/UtauTTS/internal/plan"
)

func TestLegacyJapaneseContinuousMixOnlyUsesOrdinaryJapanesePlans(t *testing.T) {
	cases := []struct {
		name string
		plan *plan.Plan
		want bool
	}{
		{name: "japanese", plan: &plan.Plan{Language: "ja", Phonemizer: "ja-kana"}, want: true},
		{name: "single-cv", plan: &plan.Plan{Language: "ja", SingleCV: true}, want: false},
		{name: "english", plan: &plan.Plan{Language: "en", Phonemizer: "en-vccv"}, want: false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := legacyJapaneseContinuousMix(test.plan); got != test.want {
				t.Fatalf("legacy mix = %v, want %v", got, test.want)
			}
		})
	}
}
