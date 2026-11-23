package zh

import (
	"reflect"
	"strings"
	"testing"

	"github.com/yh2237/UtauTTS/internal/linguistics"
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
		got, err := Parse(text, "", nil)
		if err != nil {
			t.Fatalf("%q: %v", text, err)
		}
		if !strings.HasPrefix(got.Reading, want) {
			t.Errorf("%q: reading %q does not start with %q", text, got.Reading, want)
		}
		if len(got.Morae) == 0 {
			t.Errorf("%q: no morae", text)
		}
	}
}

func TestChineseAllLetterNamesAreReadable(t *testing.T) {
	if _, err := Parse("ABCDEFGHIJKLMNOPQRSTUVWXYZ", "", nil); err != nil {
		t.Fatalf("%v", err)
	}
}

func TestChineseUnknownLatinFallsBackToLetterNames(t *testing.T) {
	got, err := Parse("XYZ测试。", "", nil)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if !strings.HasPrefix(got.Reading, "ai4 ke4 si1 wai1 zei2 ") {
		t.Fatalf("reading = %q", got.Reading)
	}
}

func TestChineseLatinReading(t *testing.T) {
	tests := map[string]string{
		"AI":  "ei1 ai4",
		"GDP": "ji1 di1 pi1",
		"T":   "ti1",
	}
	for input, want := range tests {
		if got := ChineseLatinReading(input); got != want {
			t.Errorf("ChineseLatinReading(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestChineseFamilyTermsUseNeutralTone(t *testing.T) {
	for _, tc := range []struct {
		text    string
		reading string
		tone    int
	}{
		{"爷爷", "ye2 ye5", 2},
		{"爺爺", "ye2 ye5", 2},
		{"奶奶", "nai3 nai5", 3},
		{"妈妈", "ma1 ma5", 1},
		{"爸爸", "ba4 ba5", 4},
	} {
		t.Run(tc.text, func(t *testing.T) {
			got, err := Parse(tc.text+"。", "", nil)
			if err != nil {
				t.Fatal(err)
			}
			if got.Reading != tc.reading+" |" || len(got.Morae) != 3 {
				t.Fatalf("reading = %q, units = %d", got.Reading, len(got.Morae))
			}
			if got.Morae[0].Tone != tc.tone || got.Morae[1].Tone != 5 || !got.Morae[2].Pause {
				t.Fatalf("tones or pause = %+v", got.Morae)
			}
			if got.Morae[0].WordEnd || !got.Morae[1].WordEnd || got.Morae[0].WordIndex != got.Morae[1].WordIndex {
				t.Fatalf("word boundary = %+v", got.Morae)
			}
			for i, character := range []rune(tc.text) {
				if got.Morae[i].SourceText != string(character) {
					t.Fatalf("source character %d = %q", i, got.Morae[i].SourceText)
				}
			}
		})
	}
	got, err := Parse("爷爷和奶奶。", "", nil)
	if err != nil || got.Reading != "ye2 ye5 he2 nai3 nai5 |" {
		t.Fatalf("embedded terms = %q, %v", got.Reading, err)
	}
}

func TestChinesePercentAndNumbers(t *testing.T) {
	got, err := Parse("50%的人同意。", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Reading, "bai3 fen1 zhi1 wu3 shi2") {
		t.Fatalf("reading = %q", got.Reading)
	}
	for input, want := range map[string]string{"2026年9月9日": "二零二六年九月九日", "101": "一百零一", "110": "一百一十", "10010": "一万零一十", "3.14": "三点一四"} {
		if norm := Normalize(input); norm != want {
			t.Errorf("%q: %q want %q", input, norm, want)
		}
	}
}

func TestChineseFamilyTermsPreserveExplicitPronunciation(t *testing.T) {
	for _, tc := range []struct {
		text       string
		explicit   string
		dictionary map[string]string
		want       string
	}{
		{"爷爷。", "", map[string]string{"爷爷": "ye2 ye2"}, "ye2 ye2 |"},
		{"奶奶。", "", map[string]string{"奶奶": "nai3 nai3"}, "nai3 nai3 |"},
		{"爷爷。", "ye2 ye1 |", map[string]string{"爷爷": "ye2 ye4"}, "ye2 ye1 |"},
		{"爺。", "", nil, "ye2 |"},
		{"奶。", "", nil, "nai3 |"},
	} {
		got, err := Parse(tc.text, tc.explicit, tc.dictionary)
		if err != nil || got.Reading != tc.want {
			t.Errorf("%q explicit %q: got %q, %v, want %q", tc.text, tc.explicit, got.Reading, err, tc.want)
		}
	}
}

func TestChineseWordReadingWhitespaceAndLongestDictionary(t *testing.T) {
	got, err := Parse("银行行长来了。", "", nil)
	if err != nil || got.Reading != "yin2 hang2 hang2 zhang3 lai2 le5 |" {
		t.Fatalf("%q %v", got.Reading, err)
	}
	if got.Morae[0].WordEnd || !got.Morae[1].WordEnd || got.Morae[0].WordIndex != got.Morae[1].WordIndex {
		t.Fatalf("word boundaries: %+v", got.Morae)
	}
	ws, err := Parse("你好 世界", "", nil)
	if err != nil || strings.Contains(ws.Reading, "|") {
		t.Fatalf("spaces became pauses: %q %v", ws.Reading, err)
	}
	longest, err := Parse("重庆人", "", map[string]string{"重": "zhong", "重庆": "chong qing"})
	if err != nil || longest.Reading != "chong qing ren2" {
		t.Fatalf("reading = %q, %v", longest.Reading, err)
	}
}

func TestChineseRhymePartsAndNasalCodas(t *testing.T) {
	for _, tc := range []struct {
		reading string
		parts   []linguistics.Phone
	}{
		{"hao3", []linguistics.Phone{{Symbol: "h", Role: "onset"}, {Symbol: "a", Role: "nucleus"}, {Symbol: "u", Role: "offglide"}}},
		{"tian1", []linguistics.Phone{{Symbol: "t", Role: "onset"}, {Symbol: "i", Role: "medial"}, {Symbol: "a", Role: "nucleus"}, {Symbol: "n", Role: "coda"}}},
		{"xiao3", []linguistics.Phone{{Symbol: "x", Role: "onset"}, {Symbol: "i", Role: "medial"}, {Symbol: "a", Role: "nucleus"}, {Symbol: "u", Role: "offglide"}}},
		{"liu2", []linguistics.Phone{{Symbol: "l", Role: "onset"}, {Symbol: "i", Role: "medial"}, {Symbol: "o", Role: "nucleus"}, {Symbol: "u", Role: "offglide"}}},
		{"gui4", []linguistics.Phone{{Symbol: "g", Role: "onset"}, {Symbol: "u", Role: "medial"}, {Symbol: "e", Role: "nucleus"}, {Symbol: "i", Role: "offglide"}}},
		{"jun1", []linguistics.Phone{{Symbol: "j", Role: "onset"}, {Symbol: "v", Role: "nucleus"}, {Symbol: "n", Role: "coda"}}},
		{"jue2", []linguistics.Phone{{Symbol: "j", Role: "onset"}, {Symbol: "v", Role: "medial"}, {Symbol: "e", Role: "nucleus"}}},
		{"ban1", []linguistics.Phone{{Symbol: "b", Role: "onset"}, {Symbol: "a", Role: "nucleus"}, {Symbol: "n", Role: "coda"}}},
		{"bang1", []linguistics.Phone{{Symbol: "b", Role: "onset"}, {Symbol: "a", Role: "nucleus"}, {Symbol: "ng", Role: "coda"}}},
		{"jin1", []linguistics.Phone{{Symbol: "j", Role: "onset"}, {Symbol: "i", Role: "nucleus"}, {Symbol: "n", Role: "coda"}}},
		{"jing1", []linguistics.Phone{{Symbol: "j", Role: "onset"}, {Symbol: "i", Role: "nucleus"}, {Symbol: "ng", Role: "coda"}}},
		{"ma1", []linguistics.Phone{{Symbol: "m", Role: "onset"}, {Symbol: "a", Role: "nucleus"}}},
	} {
		got, err := Parse("", tc.reading, nil)
		if err != nil || !reflect.DeepEqual(got.Morae[0].Phones, tc.parts) {
			t.Fatalf("%s: got %+v, err=%v", tc.reading, got.Morae, err)
		}
		if got.Morae[0].Text != NormalizePinyin(tc.reading) {
			t.Fatalf("%s: text=%q", tc.reading, got.Morae[0].Text)
		}
	}
	if NormalizePinyin("nüe4") != "nve" || PinyinTone("nüe4") != 4 {
		t.Fatalf("ü normalization failed")
	}
}

func TestChineseSurfaceReadingAndSandhi(t *testing.T) {
	for _, tc := range []struct {
		text, wantSurface string
		wantTones         []int
	}{
		{"你好。", "ni2 hao3 |", []int{2, 3, 0}},
		{"很好。", "hen2 hao3 |", []int{2, 3, 0}},
		{"我们。", "wo3 men5 |", []int{3, 5, 0}},
		{"不对", "bu2 dui4", []int{2, 4}},
		{"布对", "bu4 dui4", []int{4, 4}},
		{"一天", "yi4 tian1", []int{4, 1}},
		{"一定", "yi2 ding4", []int{2, 4}},
		{"第一天", "di4 yi1 tian1", []int{4, 1, 1}},
		{"我很好", "wo3 hen2 hao3", []int{3, 2, 3}},
		{"你好", "ni2 hao3", []int{2, 3}},
		{"你，好", "ni3 | hao3", []int{3, 0, 3}},
	} {
		got, err := Parse(tc.text, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if got.SurfaceReading != tc.wantSurface {
			t.Errorf("%s: surface = %q, want %q", tc.text, got.SurfaceReading, tc.wantSurface)
		}
		if tones := SurfaceTones(got.Morae); !reflect.DeepEqual(tones, tc.wantTones) {
			t.Errorf("%s: tones = %v, want %v", tc.text, tones, tc.wantTones)
		}
	}
	const raw = "ni3 hao3"
	if got := SurfaceReading(raw, nil); got != raw {
		t.Errorf("mismatched reading changed to %q", got)
	}
	for _, text := range []string{"我很好", "不对", "一天"} {
		first, err := Parse(text, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		second, err := Parse(text, first.Reading, nil)
		if err != nil || !reflect.DeepEqual(SurfaceTones(first.Morae), SurfaceTones(second.Morae)) {
			t.Fatalf("round trip %s: %+v %v", text, second, err)
		}
	}
}
