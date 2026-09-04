package tts

import (
	"math"
	"reflect"
	"testing"

	"utautts/internal/diffsinger"
	"utautts/internal/frontend"
)

func TestDiffSingerPhones(t *testing.T) {
	singer := &diffsinger.Singer{Tokens: map[string]int64{"SP": 0, "k": 1, "a": 2, "N": 3}}
	morae := []frontend.Mora{
		{Text: "か", Consonant: "k", Vowel: "a"},
		{Text: "ん", Vowel: "n"},
		{Pause: true},
	}
	phones, durations, counts, err := diffsingerPhones(singer, morae, []float64{100, 90, 180})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(phones, []string{"k", "a", "N", "SP"}) {
		t.Fatalf("phones = %#v", phones)
	}
	if !reflect.DeepEqual(durations, []float64{45, 55, 90, 180}) {
		t.Fatalf("durations = %#v", durations)
	}
	if !reflect.DeepEqual(counts, []int64{2, 1, 1}) {
		t.Fatalf("counts = %#v", counts)
	}
}

func TestDiffSingerConsonantDurationStrengthensFricatives(t *testing.T) {
	if got := diffsingerConsonantDuration("ja/h", 100); math.Abs(got-48) > 0.001 {
		t.Fatalf("h duration = %v", got)
	}
	if got := diffsingerConsonantDuration("ja/w", 100); got != 42 {
		t.Fatalf("w duration = %v", got)
	}
}

func TestDiffSingerPhonesUsesSingerDictionary(t *testing.T) {
	singer := &diffsinger.Singer{
		Tokens:             map[string]int64{"SP": 0, "kx": 1, "oo": 2},
		JapaneseDictionary: map[string][]string{"こ": {"kx", "oo"}},
	}
	morae := []frontend.Mora{{Text: "こ", Consonant: "k", Vowel: "o"}}
	phones, durations, counts, err := diffsingerPhones(singer, morae, []float64{100})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(phones, []string{"kx", "oo"}) || !reflect.DeepEqual(durations, []float64{45, 55}) || !reflect.DeepEqual(counts, []int64{2}) {
		t.Fatalf("phones = %#v, durations = %#v, counts = %#v", phones, durations, counts)
	}
}

func TestDiffSingerDictionaryUsesVowelForLongMark(t *testing.T) {
	singer := &diffsinger.Singer{JapaneseDictionary: map[string][]string{"お": {"oo"}}}
	got := diffsingerDictionarySymbols(singer, frontend.Mora{Text: "ー", Vowel: "o"})
	if !reflect.DeepEqual(got, []string{"oo"}) {
		t.Fatalf("symbols = %#v", got)
	}
}

func TestDurationsMSToFramesKeepsAccumulatedLength(t *testing.T) {
	got := durationsMSToFrames([]float64{80, 35, 65, 80}, 10)
	if !reflect.DeepEqual(got, []int64{8, 4, 6, 8}) {
		t.Fatalf("frames = %#v", got)
	}
}

func TestGroupedFrameDurations(t *testing.T) {
	got := groupedFrameDurations([]int64{8, 3, 7, 5, 8}, []int64{1, 2, 1, 1})
	if !reflect.DeepEqual(got, []int64{8, 10, 5, 8}) {
		t.Fatalf("durations = %#v", got)
	}
}

func TestDiffSingerMelScale(t *testing.T) {
	if got := diffsingerMelScale("10", "e"); got < 2.3025 || got > 2.3026 {
		t.Fatalf("scale = %v", got)
	}
}
