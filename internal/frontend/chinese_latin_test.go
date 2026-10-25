package frontend

import (
	"strings"
	"testing"
)

func TestChineseReadsLatinWordsWithoutError(t *testing.T) {
	cases := map[string]string{
		"AI技术发展很快。":     "ei1 ai4 ji4 shu4",
		"iPhone和WiFi。":  "ai4 feng1 he2 wai1 fei1",
		"GDP增长了。":       "ji1 di1 pi1 zeng1 zhang3",
		"T恤很便宜。":        "ti1 xu4 hen3 pian2 yi5",
		"OpenAI发布了新模型。": "ou1 pen1 ei1 ai4 fa1 bu4",
	}
	for text, want := range cases {
		reading, morae, err := ParseChineseCVVC(text, "", nil)
		if err != nil {
			t.Fatalf("%q: %v", text, err)
		}
		if !strings.HasPrefix(reading, want) {
			t.Errorf("%q: reading %q does not start with %q", text, reading, want)
		}
		if len(morae) == 0 {
			t.Errorf("%q: no morae", text)
		}
	}
}

func TestChineseAllLetterNamesAreReadable(t *testing.T) {
	if _, _, err := ParseChineseCVVC("ABCDEFGHIJKLMNOPQRSTUVWXYZ", "", nil); err != nil {
		t.Fatalf("%v", err)
	}
}

func TestChineseUnknownLatinFallsBackToLetterNames(t *testing.T) {
	reading, _, err := ParseChineseCVVC("XYZ测试。", "", nil)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if !strings.HasPrefix(reading, "ai4 ke4 si1 wai1 zei2 ") {
		t.Fatalf("reading = %q", reading)
	}
}
