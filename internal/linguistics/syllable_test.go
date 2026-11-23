package linguistics

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseSyllableReading(t *testing.T) {
	morae, pieces, err := ParseSyllableReading(" , ˈo.la , , kˈe tˈal ? !", LanguageSpanish)
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, mora := range morae {
		texts = append(texts, mora.Text)
	}
	if got := strings.Join(texts, " "); got != "o la , ke tal" {
		t.Fatalf("morae = %q, want %q", got, "o la , ke tal")
	}
	if morae[0].Stress != 1 || morae[1].Stress != 0 || !morae[2].Pause || morae[2].PauseKind != PauseKindComma {
		t.Fatalf("morae = %+v", morae)
	}
	if morae[4].Consonant != "t" || morae[4].Vowel != "al" {
		t.Fatalf("last mora = %+v", morae[4])
	}
	wantPieces := []SyllablePiece{
		{Symbols: "ˈo", Part: 0},
		{Symbols: "la", Part: 1},
		{Symbols: ",", Part: 2},
		{Symbols: " ", Part: -1},
		{Symbols: "kˈe", Part: 3},
		{Symbols: " ", Part: -1},
		{Symbols: "tˈal", Part: 4},
		{Symbols: "?", Part: -1},
		{Symbols: "!", Part: -1},
	}
	if !reflect.DeepEqual(pieces, wantPieces) {
		t.Fatalf("pieces = %+v, want %+v", pieces, wantPieces)
	}
}

func TestParseSyllableReadingErrors(t *testing.T) {
	for _, reading := range []string{"", " . ", "ˈo..la"} {
		if _, _, err := ParseSyllableReading(reading, LanguageSpanish); err == nil {
			t.Fatalf("ParseSyllableReading(%q) succeeded, want error", reading)
		}
	}
}
