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
		{Pause: true},
		{Text: "きょ", Consonant: "ky", Vowel: "o"},
		{Text: "う", Vowel: "u"},
		{Pause: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("morae = %#v, want %#v", got, want)
	}
}

func TestParseKanaLongVowel(t *testing.T) {
	got, err := ParseKana("スーパー")
	if err != nil {
		t.Fatal(err)
	}
	if got[1] != (Mora{Text: "ー", Vowel: "u"}) || got[3] != (Mora{Text: "ー", Vowel: "a"}) {
		t.Fatalf("morae = %#v", got)
	}
}

func TestParseKanaConsonants(t *testing.T) {
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

func TestParseKanaIgnoresUnknownCharacter(t *testing.T) {
	got, err := ParseKana("あ🙂Aい")
	if err != nil {
		t.Fatal(err)
	}
	want := []Mora{{Text: "あ", Vowel: "a"}, {Text: "い", Vowel: "i"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("morae = %#v, want %#v", got, want)
	}
}
