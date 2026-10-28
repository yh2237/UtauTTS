//go:build !js

package tts

import (
	"math"
	"reflect"
	"testing"

	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/refspeech"
)

func TestEnglishMoraTokensAndDurations(t *testing.T) {
	reading := "HH AH0 L OW1 | SP | W ER1 L D | SP"
	pieces := []refspeech.Piece{
		{Symbols: "həlˈO", Part: 0}, {Symbols: ",", Part: 1}, {Symbols: " ", Part: -1},
		{Symbols: "wˈɜɹld", Part: 2}, {Symbols: ".", Part: -1},
	}
	durations := make([]int, 16)
	for i := range durations {
		durations[i] = 2
	}
	timing := refspeech.Timing{Durations: durations, PieceTokens: [][2]int{{1, 6}, {6, 7}, {7, 8}, {8, 14}, {14, 15}}}
	two := []frontend.Phone{{Symbol: "a"}, {Symbol: "b"}}
	morae := []frontend.Mora{
		{WordIndex: 0, Phones: two}, {WordIndex: 0, Phones: two}, {Pause: true},
		{WordIndex: 2, Phones: append(two, two...)}, {Pause: true},
	}
	tokens := englishMoraTokens(pieces, timing, reading, morae)
	if !reflect.DeepEqual(tokens, [][]int{{1, 2, 3}, {4, 5}, {6, 7}, {8, 9, 10, 11, 12, 13}, {14, 15}}) {
		t.Fatalf("tokens = %v", tokens)
	}
	got := referenceMoraDurations(tokens, timing, []float64{0, 0, 0, 250})
	if !reflect.DeepEqual(got, []float64{150, 100, 100, 250, 100}) {
		t.Fatalf("durations = %v", got)
	}
}

// 音素の指定があるモーラは、その合計がモーラの長さになる。
func TestReferencePlannedDurationsUsesPhonePlanTotal(t *testing.T) {
	morae := []frontend.Mora{{Phones: []frontend.Phone{{Symbol: "k", Role: "onset"}, {Symbol: "a", Role: "nucleus"}}}}
	timing := refspeech.Timing{Durations: []int{2, 6}}
	tokens := [][]int{{0, 1}}
	phonePlans := referencePhonePlans(morae, tokens, timing, []float64{250}, nil)
	got := referencePlannedDurations(tokens, timing, []float64{250}, phonePlans)
	if len(got) != 1 || math.Abs(got[0]-250) > 1e-6 {
		t.Fatalf("planned durations = %v, want 250", got)
	}
}

func TestBuildPhonePlansSplitsConsonantAndVowel(t *testing.T) {
	morae := []frontend.Mora{
		{Phones: []frontend.Phone{{Symbol: "k", Role: "onset"}, {Symbol: "a", Role: "nucleus"}, {Symbol: "n", Role: "coda"}}},
		{Pause: true},
	}
	cfg := Config{ConsonantMS: []float64{80, 0}, VowelMS: []float64{160, 0}}
	plans, err := buildPhonePlans(cfg, morae)
	if err != nil {
		t.Fatal(err)
	}
	if plans[0].consonantMS != 80 || plans[0].vowelMS != 160 {
		t.Fatalf("plans = %v", plans)
	}
	if plans[1].consonantMS != 0 || plans[1].vowelMS != 0 {
		t.Fatalf("pause plan = %v", plans[1])
	}
}

func TestEnglishPitchCurveStartsAtFirstMoraAndBridgesUnvoiced(t *testing.T) {
	f0 := []float64{0, 0, 200, 0, 0, 400, 0}
	curve := referencePitchCurve(f0, 200, 2*refspeech.F0FrameMS, 1)
	want := []float64{0, 400, 800, 1200, 1200}
	if curve == nil || len(curve.Cents) != len(want) {
		t.Fatalf("curve = %+v", curve)
	}
	for i := range want {
		if math.Abs(curve.Cents[i]-want[i]) > 1e-9 {
			t.Fatalf("cents = %v, want %v", curve.Cents, want)
		}
	}
	if referencePitchCurve([]float64{0, 0}, 200, 0, 1) != nil {
		t.Fatal("unvoiced reference produced a curve")
	}
}
