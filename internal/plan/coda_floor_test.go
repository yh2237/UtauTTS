package plan

import (
	"math"
	"testing"

	"utautts/internal/frontend"
)

func englishCodaMora() frontend.Mora {
	return frontend.Mora{Language: frontend.LanguageEnglish, Phones: []frontend.Phone{
		{Symbol: "m", Role: "onset"},
		{Symbol: "iy", Role: "nucleus"},
		{Symbol: "t", Role: "coda"},
	}}
}

func TestEnglishCodaFloorReservesStopClosure(t *testing.T) {
	t.Run("stop closure floor", func(t *testing.T) {
		start, span, floor := speechEndingTiming(englishCodaMora(), []float64{32.2, 72.2, 37.2}, 0, 0, 141.6)
		if math.Abs(span-englishCodaMinStopMS) > 1e-9 || math.Abs(floor-englishCodaMinStopMS) > 1e-9 {
			t.Fatalf("span=%v floor=%v", span, floor)
		}
		if math.Abs(start+span-141.6) > 1e-9 {
			t.Fatalf("coda must end at mora end: start=%v span=%v", start, span)
		}
		if start < englishCodaMinVowelMS {
			t.Fatalf("vowel floor violated: start=%v", start)
		}
	})
	t.Run("long coda unchanged", func(t *testing.T) {
		start, span, floor := speechEndingTiming(englishCodaMora(), []float64{30, 60, 90}, 0, 0, 180)
		if start != 90 || span != 90 || floor != 0 {
			t.Fatalf("long coda changed: start=%v span=%v floor=%v", start, span, floor)
		}
	})
	t.Run("other languages skipped", func(t *testing.T) {
		mora := englishCodaMora()
		mora.Language = frontend.LanguageJapanese
		start, span, floor := speechEndingTiming(mora, []float64{32.2, 72.2, 37.2}, 0, 0, 141.6)
		if math.Abs(start-104.4) > 1e-9 || math.Abs(span-37.2) > 1e-9 || floor != 0 {
			t.Fatalf("non-English changed: start=%v span=%v floor=%v", start, span, floor)
		}
	})
}

func TestEnglishCodaFloorClampsToVowelMinimum(t *testing.T) {
	mora := englishCodaMora()
	// 母音の最低長を残すため、codaは55msまでしか伸ばせない。
	start, span, floor := speechEndingTiming(mora, []float64{20, 50, 30}, 0, 0, 100)
	if math.Abs(span-55) > 1e-9 || math.Abs(start-45) > 1e-9 || math.Abs(floor-55) > 1e-9 {
		t.Fatalf("clamp failed: start=%v span=%v floor=%v", start, span, floor)
	}
	shortStart, shortSpan, shortFloor := speechEndingTiming(mora, []float64{20, 25, 30}, 0, 0, 75)
	if math.Abs(shortStart-45) > 1e-9 || math.Abs(shortSpan-30) > 1e-9 || shortFloor != 0 {
		t.Fatalf("short mora changed: start=%v span=%v floor=%v", shortStart, shortSpan, shortFloor)
	}
}
