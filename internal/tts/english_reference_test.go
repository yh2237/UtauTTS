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
	got := englishMoraDurations(tokens, timing, []float64{0, 0, 0, 250})
	if !reflect.DeepEqual(got, []float64{150, 100, 100, 250, 100}) {
		t.Fatalf("durations = %v", got)
	}
	forced := englishForcedDurations(timing, tokens, []float64{0, 200, 0, 0, 0})
	if forced[4] != 4 || forced[5] != 4 || forced[1] != 2 {
		t.Fatalf("forced = %v", forced)
	}
	shorter := englishForcedDurations(timing, tokens, []float64{0, 0, 0, 260})
	total := 0
	for _, token := range tokens[3] {
		total += shorter[token]
	}
	if total != 10 {
		t.Fatalf("forced word frames = %d, want 10", total)
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

func TestEnglishAliasPhones(t *testing.T) {
	for _, test := range []struct {
		alias string
		want  []string
	}{
		{"aa", []string{"aa"}},
		{"ae1", []string{"ae"}},
		{"th_ax", []string{"th", "ax"}},
		{"- aa", []string{"aa"}},
		{"oy-", []string{"oy"}},
		{"_3_h3_3-", nil},
		{"ra_ri_ru_re_ro", nil},
		{"", nil},
	} {
		if got := englishAliasPhones(test.alias); !reflect.DeepEqual(got, test.want) {
			t.Errorf("englishAliasPhones(%q) = %v, want %v", test.alias, got, test.want)
		}
	}
}
