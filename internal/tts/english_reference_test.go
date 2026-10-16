//go:build !js

package tts

import (
	"math"
	"testing"

	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/refspeech"
)

func TestEnglishMoraDurationsSplitWordsAndPauses(t *testing.T) {
	reading := "HH AH0 L OW1 | SP | W ER1 L D | SP"
	pieces := []refspeech.Piece{
		{Symbols: "həlˈO", Part: 0}, {Symbols: ",", Part: 1}, {Symbols: " ", Part: -1},
		{Symbols: "wˈɜɹld", Part: 2}, {Symbols: ".", Part: -1},
	}
	pieceMS := []float64{200, 100, 25, 300, 50}
	morae := []frontend.Mora{
		{WordIndex: 0, Text: "he"}, {WordIndex: 0, Text: "lo"}, {Pause: true},
		{WordIndex: 2, Text: "world"}, {Pause: true},
	}
	got := englishMoraDurations(pieces, pieceMS, 75, reading, morae)
	want := []float64{100, 100, 125, 300, 125}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Fatalf("durations = %v, want %v", got, want)
		}
	}
}

func TestEnglishPitchCurveStartsAtFirstMoraAndBridgesUnvoiced(t *testing.T) {
	f0 := []float64{0, 0, 200, 0, 0, 400, 0}
	curve := englishPitchCurve(f0, 200, 2*refspeech.F0FrameMS, 1)
	want := []float64{0, 400, 800, 1200, 1200}
	if curve == nil || len(curve.Cents) != len(want) {
		t.Fatalf("curve = %+v", curve)
	}
	for i := range want {
		if math.Abs(curve.Cents[i]-want[i]) > 1e-9 {
			t.Fatalf("cents = %v, want %v", curve.Cents, want)
		}
	}
	if englishPitchCurve([]float64{0, 0}, 200, 0, 1) != nil {
		t.Fatal("unvoiced reference produced a curve")
	}
}
