package adapter

import (
	"math"
	"reflect"
	"testing"

	"utautts/internal/diffsinger"
	"utautts/internal/frontend"
	"utautts/internal/prosody"
	"utautts/internal/render"
)

func TestDiffSingerPhones(t *testing.T) {
	singer := &diffsinger.Singer{Tokens: map[string]int64{"SP": 0, "k": 1, "a": 2, "N": 3}}
	morae := []frontend.Mora{
		{Text: "か", Consonant: "k", Vowel: "a"},
		{Text: "ん", Vowel: "n"},
		{Pause: true},
	}
	phones, durations, counts, err := diffsingerPhones(singer, morae, []float64{100, 90, 180}, nil)
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
	phones, durations, counts, err := diffsingerPhones(singer, morae, []float64{100}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(phones, []string{"kx", "oo"}) || !reflect.DeepEqual(durations, []float64{45, 55}) || !reflect.DeepEqual(counts, []int64{2}) {
		t.Fatalf("phones = %#v, durations = %#v, counts = %#v", phones, durations, counts)
	}
}

func TestDiffSingerConsonantDurationUsesLongerOfTimelineAndRatio(t *testing.T) {
	singer := &diffsinger.Singer{Tokens: map[string]int64{"SP": 0, "k": 1, "a": 2}}
	morae := []frontend.Mora{{Text: "か", Consonant: "k", Vowel: "a", Phones: []frontend.Phone{
		{Symbol: "k", Role: "onset"}, {Symbol: "a", Role: "nucleus"},
	}}}
	// 共有重み(35ms)より話声向け比率(60.75ms)が長いので比率を採る。
	phones, durations, _, err := diffsingerPhones(singer, morae, []float64{135}, [][]float64{{.35, 1}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(phones, []string{"k", "a"}) || math.Abs(durations[0]-60.75) > .001 || math.Abs(durations[1]-74.25) > .001 {
		t.Fatalf("phones=%v durations=%v", phones, durations)
	}
	// 共有重みのほうが長い場合は重みを採る。
	_, durations, _, err = diffsingerPhones(singer, morae, []float64{135}, [][]float64{{1.2, 1}})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(durations[0]-135*1.2/2.2) > .001 {
		t.Fatalf("durations=%v", durations)
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

func TestDiffSingerWordGroupsUsesOpenJTalkWordEnd(t *testing.T) {
	morae := []frontend.Mora{{Text: "こ"}, {Text: "ん"}, {Text: "に"}, {Text: "ち"}, {Text: "は"}, {Pause: true}, {Text: "せ"}, {Text: "か"}, {Text: "い"}}
	counts := []int64{2, 1, 2, 2, 2, 1, 2, 2, 1}
	features := []prosody.FeatureFrame{{}, {"word_end": 1}, {}, {}, {"word_end": 1}, {}, {}, {}, {"word_end": 1}}
	groups, rests := diffsingerWordGroups(morae, counts, features)
	if !reflect.DeepEqual(groups, []int64{1, 3, 6, 1, 5, 1}) {
		t.Fatalf("groups = %v", groups)
	}
	if !reflect.DeepEqual(rests, []bool{true, false, false, true, false, true}) {
		t.Fatalf("rests = %v", rests)
	}
}

func TestDiffSingerMIDICurvesFollowPitch(t *testing.T) {
	f0 := make([]float32, 0, 30)
	for i := 0; i < 10; i++ {
		f0 = append(f0, 440)
	}
	for i := 0; i < 10; i++ {
		f0 = append(f0, 880)
	}
	for i := 0; i < 10; i++ {
		f0 = append(f0, 0)
	}
	notes, phones := diffsingerMIDICurves(f0, []int64{10, 20}, []int64{10, 10, 10}, 60)
	if len(notes) != 2 || len(phones) != 3 {
		t.Fatalf("lengths = %d %d", len(notes), len(phones))
	}
	if math.Abs(float64(notes[0])-69) > 0.01 || math.Abs(float64(notes[1])-81) > 0.01 {
		t.Fatalf("notes = %v", notes)
	}
	if phones[0] != 69 || phones[1] != 81 || phones[2] != 81 {
		t.Fatalf("phones = %v", phones)
	}
}

func TestDiffSingerMIDICurvesFallsBackWhenUnvoiced(t *testing.T) {
	notes, phones := diffsingerMIDICurves([]float32{0, 0}, []int64{2}, []int64{2}, 60)
	if notes[0] != 60 || phones[0] != 60 {
		t.Fatalf("notes=%v phones=%v", notes, phones)
	}
}

func TestDiffSingerWordGroupsKeepsMoraFallback(t *testing.T) {
	morae := []frontend.Mora{{Text: "あ"}, {Text: "い"}}
	groups, rests := diffsingerWordGroups(morae, []int64{1, 2}, nil)
	if !reflect.DeepEqual(groups, []int64{1, 1, 2, 1}) {
		t.Fatalf("groups = %v", groups)
	}
	if !reflect.DeepEqual(rests, []bool{true, false, false, true}) {
		t.Fatalf("rests = %v", rests)
	}
}

func TestShiftPitchCurveStartsAfterHeadPadding(t *testing.T) {
	curve := &render.PitchCurve{FrameMS: 10, Cents: []float64{0, 100, 200, 300}}
	shifted := shiftPitchCurve(curve, 10, 30)
	if got := shifted.CentsAt(diffsinger.HeadFrames*10 + 10); got != 100 {
		t.Fatalf("shifted pitch = %v", got)
	}
	if curve.Cents[1] != 100 {
		t.Fatal("input curve mutated")
	}
}
