package tts

import (
	"testing"

	"utautts/internal/frontend"
	"utautts/internal/plan"
)

func TestPreviewClosureUsesNormalMoraDuration(t *testing.T) {
	morae, err := frontend.ParseKana("あっ")
	if err != nil {
		t.Fatal(err)
	}
	for _, base := range []float64{100, 120, 200} {
		if got := plan.DurationFor(morae[1], base); got != plan.DurationFor(morae[0], base) {
			t.Fatalf("closure duration = %v, want %v", got, base)
		}
	}
}
