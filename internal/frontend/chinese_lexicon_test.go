package frontend

import (
	"reflect"
	"strings"
	"testing"
)

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
			reading, units, err := ParseChineseCVVC(tc.text+"。", "", nil)
			if err != nil {
				t.Fatal(err)
			}
			if reading != tc.reading+" |" || len(units) != 3 {
				t.Fatalf("reading = %q, units = %d", reading, len(units))
			}
			if units[0].Tone != tc.tone || units[1].Tone != 5 || !units[2].Pause {
				t.Fatalf("tones or pause = %+v", units)
			}
			if units[0].WordEnd || !units[1].WordEnd || units[0].WordIndex != units[1].WordIndex {
				t.Fatalf("word boundary = %+v", units)
			}
			for i, character := range []rune(tc.text) {
				if units[i].SourceText != string(character) {
					t.Fatalf("source character %d = %q", i, units[i].SourceText)
				}
			}
		})
	}
	reading, _, err := ParseChineseCVVC("爷爷和奶奶。", "", nil)
	if err != nil || reading != "ye2 ye5 he2 nai3 nai5 |" {
		t.Fatalf("embedded terms = %q, %v", reading, err)
	}
}

func TestChinesePercent(t *testing.T) {
	reading, _, err := ParseChineseCVVC("50%的人同意。", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reading, "bai3 fen1 zhi1 wu3 shi2") {
		t.Fatalf("reading = %q", reading)
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
		reading, _, err := ParseChineseCVVC(tc.text, tc.explicit, tc.dictionary)
		if err != nil || reading != tc.want {
			t.Errorf("%q explicit %q: got %q, %v, want %q", tc.text, tc.explicit, reading, err, tc.want)
		}
	}
}

func TestChineseFamilyToneChangesPreserveAliasesAndPhones(t *testing.T) {
	for _, tc := range []struct{ text, before string }{{"爷爷。", "ye2 ye2 |"}, {"奶奶。", "nai3 nai3 |"}} {
		_, automatic, err := ParseChineseCVVC(tc.text, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		_, before, err := ParseChineseCVVC(tc.text, tc.before, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(automatic) != len(before) {
			t.Fatal("unit count changed")
		}
		for i := range automatic {
			if !reflect.DeepEqual(automatic[i].Aliases, before[i].Aliases) || !reflect.DeepEqual(automatic[i].Phones, before[i].Phones) {
				t.Fatalf("%s unit %d changed aliases or phones", strings.TrimSuffix(tc.text, "。"), i)
			}
		}
	}
}
