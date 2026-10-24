//go:build !js

package tts

import (
	"math"
	"testing"

	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/plan"
)

func TestReferenceUnitAdjustmentsSkipPauses(t *testing.T) {
	morae := []frontend.Mora{{Text: "a"}, {Pause: true}, {Text: "i"}}
	pitch, volume := 2.0, 50
	cents, gain, err := referenceUnitAdjustments(Config{UnitOverrides: []plan.UnitOverride{
		{Index: 1, PitchFactor: &pitch, ResamplerVolume: &volume},
	}}, morae)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(cents[2]-1200) > 1e-9 || gain[2] != 0.5 || len(cents) != 1 || len(gain) != 1 {
		t.Fatalf("cents = %v, gain = %v", cents, gain)
	}
	if _, _, err := referenceUnitAdjustments(Config{UnitOverrides: []plan.UnitOverride{{Index: 2}}}, morae); err == nil {
		t.Fatal("out-of-range unit was accepted")
	}
}

func TestMoraEnvelopeRampsAtBoundaries(t *testing.T) {
	envelope := moraEnvelope([]float64{100, 100}, map[int]float64{1: 2}, 1)
	for ms, want := range map[float64]float64{50: 1, 100: 1.5, 150: 2, 90: 1, 110: 2} {
		if got := envelope(ms); math.Abs(got-want) > 1e-9 {
			t.Errorf("envelope(%v) = %v, want %v", ms, got, want)
		}
	}
	if moraEnvelope([]float64{100}, nil, 1) != nil {
		t.Fatal("empty values should give no envelope")
	}
}
