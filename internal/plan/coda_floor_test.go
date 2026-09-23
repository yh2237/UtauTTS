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
	mora := englishCodaMora()
	spans := []float64{32.2, 72.2, 37.2}
	start, span, floor := speechEndingTiming(mora, spans, 0, 0, 141.6)
	if math.Abs(span-englishCodaMinStopMS) > 1e-9 || math.Abs(floor-englishCodaMinStopMS) > 1e-9 {
		t.Fatalf("span=%v floor=%v", span, floor)
	}
	// モーラ総長は変えず、先行母音から再配分する。
	if math.Abs(start+span-141.6) > 1e-9 {
		t.Fatalf("coda must end at mora end: start=%v span=%v", start, span)
	}
	if start < englishCodaMinVowelMS {
		t.Fatalf("vowel floor violated: start=%v", start)
	}
}

func TestEnglishCodaFloorLeavesLongCodaUnchanged(t *testing.T) {
	mora := englishCodaMora()
	spans := []float64{30, 60, 90}
	start, span, floor := speechEndingTiming(mora, spans, 0, 0, 180)
	if start != 90 || span != 90 || floor != 0 {
		t.Fatalf("long coda changed: start=%v span=%v floor=%v", start, span, floor)
	}
}

func TestEnglishCodaFloorSkipsOtherLanguages(t *testing.T) {
	mora := englishCodaMora()
	mora.Language = frontend.LanguageJapanese
	start, span, floor := speechEndingTiming(mora, []float64{32.2, 72.2, 37.2}, 0, 0, 141.6)
	if math.Abs(start-104.4) > 1e-9 || math.Abs(span-37.2) > 1e-9 || floor != 0 {
		t.Fatalf("non-English changed: start=%v span=%v floor=%v", start, span, floor)
	}
}

func TestEnglishCodaFloorClampsToVowelMinimum(t *testing.T) {
	mora := englishCodaMora()
	// 母音の最低長を残すため、codaは55msまでしか伸ばせない。
	start, span, floor := speechEndingTiming(mora, []float64{20, 50, 30}, 0, 0, 100)
	if math.Abs(span-55) > 1e-9 || math.Abs(start-45) > 1e-9 || math.Abs(floor-55) > 1e-9 {
		t.Fatalf("clamp failed: start=%v span=%v floor=%v", start, span, floor)
	}
	// 上限がcoda長を下回る場合は何もしない。
	shortStart, shortSpan, shortFloor := speechEndingTiming(mora, []float64{20, 25, 30}, 0, 0, 75)
	if math.Abs(shortStart-45) > 1e-9 || math.Abs(shortSpan-30) > 1e-9 || shortFloor != 0 {
		t.Fatalf("short mora changed: start=%v span=%v floor=%v", shortStart, shortSpan, shortFloor)
	}
}

func TestEnglishCodaFloorClassifiesPhones(t *testing.T) {
	stop := []frontend.Phone{{Symbol: "t", Role: "coda"}}
	continuant := []frontend.Phone{{Symbol: "n", Role: "coda"}}
	other := []frontend.Phone{{Symbol: "w", Role: "coda"}}
	if got := englishCodaFloorMS(stop); got != englishCodaMinStopMS {
		t.Fatalf("stop floor=%v", got)
	}
	if got := englishCodaFloorMS(continuant); got != englishCodaMinContinuantMS {
		t.Fatalf("continuant floor=%v", got)
	}
	if got := englishCodaFloorMS(other); got != 0 {
		t.Fatalf("other floor=%v", got)
	}
}
