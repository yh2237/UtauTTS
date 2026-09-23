package frontend

import (
	"reflect"
	"testing"
)

func TestParseKana(t *testing.T) {
	got, err := ParseKana("コンニチハ、きょう。")
	if err != nil {
		t.Fatal(err)
	}
	want := []Mora{
		{Text: "こ", Consonant: "k", Vowel: "o"},
		{Text: "ん", Consonant: "n", Vowel: "n"},
		{Text: "に", Consonant: "n", Vowel: "i"},
		{Text: "ち", Consonant: "ch", Vowel: "i"},
		{Text: "は", Consonant: "h", Vowel: "a"},
		{Pause: true, PauseKind: PauseKindComma},
		{Text: "きょ", Consonant: "ky", Vowel: "o"},
		{Text: "う", Vowel: "u"},
		{Pause: true, PauseKind: PauseKindPeriod},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("morae = %#v, want %#v", got, want)
	}
	t.Run("long-vowel", testParseKanaLongVowel)
	t.Run("ellipsis-and-brackets", testParseKanaEllipsisAndBrackets)
	t.Run("pause-kinds", testParseKanaPauseKinds)
	t.Run("consecutive-pause-kinds", testParseKanaConsecutivePauseKinds)
	t.Run("consonants", testParseKanaConsonants)
	t.Run("unknown-character", testParseKanaIgnoresUnknownCharacter)
}

func testParseKanaLongVowel(t *testing.T) {
	got, err := ParseKana("スーパー")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got[1], Mora{Text: "ー", Vowel: "u"}) || !reflect.DeepEqual(got[3], Mora{Text: "ー", Vowel: "a"}) {
		t.Fatalf("morae = %#v", got)
	}
}

func testParseKanaEllipsisAndBrackets(t *testing.T) {
	got, err := ParseKana("ミナサン……（テスト）〜オハヨー〜")
	if err != nil {
		t.Fatal(err)
	}
	want := []Mora{
		{Text: "み", Consonant: "m", Vowel: "i"},
		{Text: "な", Consonant: "n", Vowel: "a"},
		{Text: "さ", Consonant: "s", Vowel: "a"},
		{Text: "ん", Consonant: "n", Vowel: "n"},
		{Pause: true, PauseKind: PauseKindEllipsis},
		{Text: "て", Consonant: "t", Vowel: "e"},
		{Text: "す", Consonant: "s", Vowel: "u"},
		{Text: "と", Consonant: "t", Vowel: "o"},
		{Pause: true, PauseKind: PauseKindEllipsis},
		{Text: "お", Vowel: "o"},
		{Text: "は", Consonant: "h", Vowel: "a"},
		{Text: "よ", Consonant: "y", Vowel: "o"},
		{Text: "ー", Vowel: "o"},
		{Pause: true, PauseKind: PauseKindEllipsis},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("morae = %#v, want %#v", got, want)
	}
}

func testParseKanaPauseKinds(t *testing.T) {
	cases := []struct {
		reading string
		want    string
	}{
		{"あ、い", PauseKindComma},
		{"あ，い", PauseKindComma},
		{"あ,い", PauseKindComma},
		{"あ。い", PauseKindPeriod},
		{"あ．い", PauseKindPeriod},
		{"あ.い", PauseKindPeriod},
		{"あ？い", PauseKindQuestion},
		{"あ?い", PauseKindQuestion},
		{"あ！い", PauseKindQuestion},
		{"あ!い", PauseKindQuestion},
		{"あ…い", PauseKindEllipsis},
		{"あ〜い", PauseKindEllipsis},
		{"あ い", PauseKindSpace},
		{"あ　い", PauseKindSpace},
		{"あ（い）う", PauseKindBracket},
		{"あ・い", PauseKindOther},
	}
	for _, testCase := range cases {
		morae, err := ParseKana(testCase.reading)
		if err != nil {
			t.Fatalf("ParseKana(%q): %v", testCase.reading, err)
		}
		var found string
		for _, mora := range morae {
			if mora.Pause {
				found = mora.PauseKind
				break
			}
		}
		if found != testCase.want {
			t.Errorf("ParseKana(%q) pause kind = %q, want %q", testCase.reading, found, testCase.want)
		}
	}
}

// 連続する句読点は1つのポーズにまとめ、優先度が高い種類を採用する。
func testParseKanaConsecutivePauseKinds(t *testing.T) {
	cases := []struct {
		reading string
		want    string
	}{
		{"あ、。い", PauseKindPeriod},
		{"あ。？い", PauseKindQuestion},
		{"あ、？い", PauseKindQuestion},
		{"あ、…い", PauseKindEllipsis},
		{"あ。…い", PauseKindEllipsis},
		{"あ　、い", PauseKindComma},
		{"あ（、）い", PauseKindComma},
	}
	for _, testCase := range cases {
		morae, err := ParseKana(testCase.reading)
		if err != nil {
			t.Fatalf("ParseKana(%q): %v", testCase.reading, err)
		}
		pauses := 0
		var found string
		for _, mora := range morae {
			if mora.Pause {
				pauses++
				found = mora.PauseKind
			}
		}
		if pauses != 1 {
			t.Errorf("ParseKana(%q) produced %d pauses, want 1", testCase.reading, pauses)
		}
		if found != testCase.want {
			t.Errorf("ParseKana(%q) pause kind = %q, want %q", testCase.reading, found, testCase.want)
		}
	}
}

func testParseKanaConsonants(t *testing.T) {
	got, err := ParseKana("かしゃつきょんっ")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"k", "sh", "ts", "ky", "n", "cl"}
	if len(got) != len(want) {
		t.Fatalf("morae = %#v, want %d morae", got, len(want))
	}
	for index, consonant := range want {
		if got[index].Consonant != consonant {
			t.Errorf("mora %q consonant = %q, want %q", got[index].Text, got[index].Consonant, consonant)
		}
	}
	if got := ConsonantOf("キャ"); got != "ky" {
		t.Fatalf("katakana consonant = %q, want ky", got)
	}
}

func testParseKanaIgnoresUnknownCharacter(t *testing.T) {
	got, err := ParseKana("あ🙂Aい")
	if err != nil {
		t.Fatal(err)
	}
	want := []Mora{{Text: "あ", Vowel: "a"}, {Text: "い", Vowel: "i"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("morae = %#v, want %#v", got, want)
	}
}
