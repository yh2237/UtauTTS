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
	plans := []phonePlan{{weights: []float64{0.5, 0.5}}}
	phonePlans := referencePhonePlans(morae, tokens, timing, []float64{0, 200, 0, 0, 0}, plans)
	forced, err := referenceForcedDurations(timing, tokens, morae, phonePlans)
	if err != nil {
		t.Fatal(err)
	}
	if forced[4] != 4 || forced[5] != 4 || forced[1] != 2 {
		t.Fatalf("forced = %v", forced)
	}
	shorterPlans := []phonePlan{{weights: []float64{0.5, 0.5}}}
	shorterPhonePlans := referencePhonePlans(morae, tokens, timing, []float64{0, 0, 0, 260}, shorterPlans)
	shorter, err := referenceForcedDurations(timing, tokens, morae, shorterPhonePlans)
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, token := range tokens[3] {
		total += shorter[token]
	}
	if total != 10 {
		t.Fatalf("forced word frames = %d, want 10", total)
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

func TestReferencePhoneWeightsSplitFramesByPhone(t *testing.T) {
	morae := []frontend.Mora{{Phones: []frontend.Phone{{Symbol: "k", Role: "onset"}, {Symbol: "a", Role: "nucleus"}}}}
	timing := refspeech.Timing{Durations: []int{5, 5}}
	plans := []phonePlan{{weights: []float64{0.2, 0.8}}}
	phonePlans := referencePhonePlans(morae, [][]int{{0, 1}}, timing, nil, plans)
	forced, err := referenceForcedDurations(timing, [][]int{{0, 1}}, morae, phonePlans)
	if err != nil {
		t.Fatal(err)
	}
	if forced[0] != 2 || forced[1] != 8 {
		t.Fatalf("forced = %v, want consonant 2 and vowel 8", forced)
	}
}

func TestReferenceMoraLengthKeepsConsonantAndStretchesVowel(t *testing.T) {
	morae := []frontend.Mora{{Phones: []frontend.Phone{{Symbol: "k", Role: "onset"}, {Symbol: "a", Role: "nucleus"}}}}
	timing := refspeech.Timing{Durations: []int{2, 6}}
	// モーラ全体を250msへ伸ばす。子音50msはそのまま、母音は200msになる。
	phonePlans := referencePhonePlans(morae, [][]int{{0, 1}}, timing, []float64{250}, nil)
	forced, err := referenceForcedDurations(timing, [][]int{{0, 1}}, morae, phonePlans)
	if err != nil {
		t.Fatal(err)
	}
	if forced[0] != 2 {
		t.Fatalf("consonant frames = %d, want 2", forced[0])
	}
	if forced[1] != 8 {
		t.Fatalf("vowel frames = %d, want 8", forced[1])
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
