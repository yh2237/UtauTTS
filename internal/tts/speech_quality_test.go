package tts

import (
	"reflect"
	"testing"

	"github.com/yh2237/UtauTTS/internal/frontend"
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
