package linguistics

import (
	"math"
	"testing"
)

func TestMandarinRhymeSharesSumToOne(t *testing.T) {
	cases := [][]Phone{
		{{Symbol: "h", Role: "onset"}, {Symbol: "a", Role: "nucleus"}, {Symbol: "u", Role: "offglide"}},
		{{Symbol: "t", Role: "onset"}, {Symbol: "i", Role: "medial"}, {Symbol: "a", Role: "nucleus"}, {Symbol: "n", Role: "coda"}},
		{{Symbol: "l", Role: "onset"}, {Symbol: "i", Role: "medial"}, {Symbol: "o", Role: "nucleus"}, {Symbol: "u", Role: "offglide"}},
		{{Symbol: "j", Role: "onset"}, {Symbol: "v", Role: "nucleus"}, {Symbol: "n", Role: "coda"}},
	}
	for _, phones := range cases {
		shares := MandarinRhymeShares(phones)
		sum := 0.0
		for _, v := range shares {
			sum += v
		}
		if math.Abs(sum-1) > 1e-9 {
			t.Fatalf("shares %v sum = %v, want 1", shares, sum)
		}
	}
}

func TestPhoneSpans(t *testing.T) {
	phones := []Phone{{Symbol: "s", Role: "onset"}, {Symbol: "a", Role: "nucleus"}}
	spans := PhoneSpans(phones, 165)
	if len(spans) != 2 || math.Abs(spans[0]-65) > 1e-6 || math.Abs(spans[1]-100) > 1e-6 {
		t.Fatalf("spans = %v, want [65 100]", spans)
	}
	weights := PhoneSpansFromWeights([]float64{1, 3}, 200)
	if math.Abs(weights[0]-50) > 1e-6 || math.Abs(weights[1]-150) > 1e-6 {
		t.Fatalf("weights = %v", weights)
	}
	empty := PhoneSpansFromWeights([]float64{0, -1, math.NaN()}, 200)
	if empty[0] != 0 || empty[1] != 0 || empty[2] != 0 {
		t.Fatalf("non-positive weights = %v", empty)
	}
}
