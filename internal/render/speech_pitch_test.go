package render

import (
	"math"
	"reflect"
	"testing"
	"utautts/internal/plan"
)

func TestSpeechPitchReferenceExcludesTransitionsAndPreservesControls(t *testing.T) {
	p := &plan.Plan{Units: []plan.Unit{{Role: "mora", NoteStartMS: 0, PitchFactor: 1}, {Role: "transition", NoteStartMS: 50, PitchFactor: 1}, {Role: "mora", NoteStartMS: 100, PitchFactor: 1}, {Role: "ending", NoteStartMS: 150, PitchFactor: 1}}}
	pitches := []float64{180, 380, 220, 0}
	factors, reference := speechReferencePitchFactors(p, pitches, 300)
	if reference != 200 {
		t.Fatal("non-vowel recordings moved register", reference)
	}
	curve := worldlineF0CurveAt(p, pitches, factors, reference, 25, 10)
	for _, hz := range curve {
		if math.Abs(hz-200) > 1e-8 {
			t.Fatal("recording pitch leaked into target", hz)
		}
	}
	p.Units[2].PitchFactor = 1.2
	factors, _ = speechReferencePitchFactors(p, pitches, 300)
	if math.Abs(pitches[2]*factors[2]-240) > 1e-8 {
		t.Fatal("manual factor lost")
	}
	if !reflect.DeepEqual(pitches, []float64{180, 380, 220, 0}) {
		t.Fatal("pitch input changed")
	}
}
