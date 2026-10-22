package tts

import (
	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/plugin"
	"reflect"
	"testing"
)

func TestChineseSandhiUsesWordsAndCharacters(t *testing.T) {
	for _, tc := range []struct {
		text string
		want []int
	}{
		{"不对", []int{2, 4}}, {"布对", []int{4, 4}}, {"一天", []int{4, 1}}, {"一定", []int{2, 4}}, {"第一天", []int{4, 1, 1}},
		{"我很好", []int{3, 2, 3}}, {"你好", []int{2, 3}}, {"你，好", []int{3, 0, 3}},
	} {
		_, m, err := frontend.ParseChineseCVVC(tc.text, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := mandarinSurfaceTones(m); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: %v want %v", tc.text, got, tc.want)
		}
	}
}

func TestChinesePreviewReadingRoundTripKeepsLexicalContext(t *testing.T) {
	for _, text := range []string{"我很好", "不对", "一天"} {
		reading, first, err := frontend.ParseChineseCVVC(text, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		_, second, err := frontend.ParseChineseCVVC(text, reading, nil)
		if err != nil || !reflect.DeepEqual(mandarinSurfaceTones(first), mandarinSurfaceTones(second)) {
			t.Fatalf("round trip %s: %+v %v", text, second, err)
		}
	}
}

func TestDiffSingerUsesJapaneseSpeechRhythmAndKeepsManualDurations(t *testing.T) {
	// DiffSinger manifestはWindows限定のため、capabilityは明示して検証する。
	internalTiming := &plugin.Capabilities{InternalTiming: true}
	baseline, err := PredictProsody(Config{Reading: "カサ", Renderer: "diffsinger", RendererCapabilities: internalTiming, MoraDurationMS: 120})
	if err != nil {
		t.Fatal(err)
	}
	plain, err := PredictProsody(Config{Reading: "カサ", MoraDurationMS: 120})
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(baseline.MoraDurationsMS, plain.MoraDurationsMS) {
		t.Fatalf("DiffSinger rhythm = %v", baseline.MoraDurationsMS)
	}
	manual := Config{Reading: "カサ", Renderer: "diffsinger", RendererCapabilities: internalTiming, MoraDurationMS: 120, MoraDurationsMS: []float64{90, 100}}
	preview, err := PredictProsody(manual)
	if err != nil || !reflect.DeepEqual(preview.MoraDurationsMS, manual.MoraDurationsMS) {
		t.Fatalf("manual: %+v %v", preview, err)
	}
}
