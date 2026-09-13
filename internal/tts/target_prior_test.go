package tts

import (
	"math"
	"testing"

	"utautts/internal/frontend"
	"utautts/internal/jsut"
)

func TestTargetPriorPhoneWeightsUsesContextAndPreservesMoraSum(t *testing.T) {
	prior := &jsut.Prior{
		Version: jsut.PriorSchemaVersion, Kind: "jsut_target_prior",
		Phones: map[string]jsut.PhonePrior{
			"k": {Count: 10, DurationMS: jsut.ScalarStats{Count: 10, Mean: 20}},
			"a": {Count: 10, DurationMS: jsut.ScalarStats{Count: 10, Mean: 80}},
		},
		Contexts: map[string]jsut.PhonePrior{
			"sil|k|a": {Count: 10, DurationMS: jsut.ScalarStats{Count: 10, Mean: 40}},
			"k|a|sil": {Count: 10, DurationMS: jsut.ScalarStats{Count: 10, Mean: 60}},
		},
	}
	morae := []frontend.Mora{{Vowel: "a", Phones: []frontend.Phone{{Symbol: "k", Role: "onset"}, {Symbol: "a", Role: "nucleus"}}}}
	weights := targetPriorPhoneWeights(prior, morae, 1, 5)
	if len(weights) != 1 || len(weights[0]) != 2 {
		t.Fatalf("weights=%v", weights)
	}
	if math.Abs(weights[0][0]-0.4) > 1e-9 || math.Abs(weights[0][1]-0.6) > 1e-9 {
		t.Fatalf("weights=%v, want [0.4 0.6]", weights[0])
	}
}

func TestTargetPriorSymbolMapsNasalOnly(t *testing.T) {
	nasal := frontend.Mora{Vowel: "n", Phones: []frontend.Phone{{Symbol: "n", Role: "nucleus"}}}
	if got := targetPriorSymbol(nasal, 0, nasal.Phones[0]); got != "N" {
		t.Fatalf("nasal symbol=%q, want N", got)
	}
	onset := frontend.Mora{Vowel: "a", Phones: []frontend.Phone{{Symbol: "n", Role: "onset"}, {Symbol: "a", Role: "nucleus"}}}
	if got := targetPriorSymbol(onset, 0, onset.Phones[0]); got != "n" {
		t.Fatalf("onset symbol=%q, want n", got)
	}
}
