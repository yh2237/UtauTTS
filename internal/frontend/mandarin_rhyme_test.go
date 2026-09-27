package frontend

import (
	"reflect"
	"testing"
)

func TestMandarinArticulationsKeepVoicebankAliases(t *testing.T) {
	for _, tc := range []struct {
		reading string
		parts   []Phone
	}{
		{"hao3", []Phone{{"h", "onset"}, {"a", "nucleus"}, {"u", "offglide"}}},
		{"tian1", []Phone{{"t", "onset"}, {"i", "medial"}, {"a", "nucleus"}, {"n", "coda"}}},
		{"xiao3", []Phone{{"x", "onset"}, {"i", "medial"}, {"a", "nucleus"}, {"u", "offglide"}}},
		{"liu2", []Phone{{"l", "onset"}, {"i", "medial"}, {"o", "nucleus"}, {"u", "offglide"}}},
		{"gui4", []Phone{{"g", "onset"}, {"u", "medial"}, {"e", "nucleus"}, {"i", "offglide"}}},
		{"jun1", []Phone{{"j", "onset"}, {"v", "nucleus"}, {"n", "coda"}}},
		{"jue2", []Phone{{"j", "onset"}, {"v", "medial"}, {"e", "nucleus"}}},
	} {
		_, m, err := ParseChineseCVVC("", tc.reading, nil)
		if err != nil || !reflect.DeepEqual(m[0].Phones, tc.parts) {
			t.Fatal(tc.reading, m, err)
		}
		if m[0].Aliases.Main[1] != normalizePinyin(tc.reading) {
			t.Fatal("alias replaced by phone fragments", m)
		}
		shares := MandarinRhymeShares(m[0].Phones)
		sum := 0.0
		for _, v := range shares {
			sum += v
		}
		if sum != 1 {
			t.Fatal("rhyme budget changed", shares)
		}
	}
}
