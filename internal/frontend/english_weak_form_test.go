package frontend

import (
	"strings"
	"testing"
)

func TestEnglishWeakForm(t *testing.T) {
	t.Run("automatic phrase interior", testOfWeakFormOnlyAutomaticPhraseInterior)
	t.Run("manual reading remains equivalent", testAutomaticOfMatchesAcceptedManualReading)
	t.Run("function words phrase interior", testEnglishWeakFormsPhraseInterior)
	t.Run("outside phrase interior", testEnglishWeakFormsRequirePhraseInterior)
	t.Run("dictionary and reading precedence", testEnglishWeakFormsPrecedence)
	t.Run("capitalization policy", testEnglishWeakFormsCapitalization)
	t.Run("disabled restores citation", testEnglishWeakFormsDisabled)
}

func testOfWeakFormOnlyAutomaticPhraseInterior(t *testing.T) {
	for _, tc := range []struct {
		text, reading string
		dict          map[string]string
		weak          bool
	}{
		{text: "Another cup of coffee.", weak: true},
		{text: "A cup OF coffee.", weak: true},
		{text: "of"}, {text: "Of course."}, {text: "What is it made of?"},
		{text: "cup, of coffee"}, {text: "cup of, coffee"},
		{text: "cup of coffee", reading: "K AH1 P | AH1 V | K AA1 F IY0"},
		{text: "cup of coffee", dict: map[string]string{"of": "AH1 V"}},
	} {
		got, _, err := englishPronunciation(tc.text, tc.reading, tc.dict)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(got, "AH0 V") != tc.weak {
			t.Fatalf("%+v: %s", tc, got)
		}
	}
}

func testAutomaticOfMatchesAcceptedManualReading(t *testing.T) {
	for _, parse := range []func(string, string, map[string]string) (string, []Mora, error){ParseEnglishDelta, ParseEnglishVCCV} {
		got, _, err := parse("Another cup of coffee.", "", nil)
		if err != nil {
			t.Fatal(err)
		}
		want := "AH0 N AH1 DH ER0 | K AH1 P | AH0 V | K AA1 F IY0 | SP"
		if got != want {
			t.Fatalf("%s != %s", got, want)
		}
	}
}

// 句中の代表的な機能語が弱形になり、引用形は残らないことを確認する。
func testEnglishWeakFormsPhraseInterior(t *testing.T) {
	for _, tc := range []struct{ text, weak, citation string }{
		{text: "bread and butter", weak: "AH0 N", citation: "AH0 N D"},
		{text: "I can do it", weak: "K AH0 N", citation: "K AE1 N"},
		{text: "back from work", weak: "F R AH0 M", citation: "F R AH1 M"},
		{text: "bread but jam", weak: "B AH0 T", citation: "B AH1 T"},
		{text: "give them the book", weak: "DH AH0 M", citation: "DH EH1 M"},
		{text: "some of us", weak: "AH0 V", citation: "AH1 V"},
		{text: "he was there", weak: "W AH0 Z", citation: "W AA1 Z"},
		{text: "they were here", weak: "W ER0", citation: "W ER1"},
		{text: "she has a cat", weak: "HH AH0 Z", citation: "HH AE1 Z"},
	} {
		got, _, err := englishPronunciation(tc.text, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, tc.weak) {
			t.Fatalf("%q: missing weak %q in %s", tc.text, tc.weak, got)
		}
		if strings.Contains(got, tc.citation) {
			t.Fatalf("%q: citation %q leaked into %s", tc.text, tc.citation, got)
		}
	}
}

// 文頭・文末・ポーズ隣接では弱形にしない。
func testEnglishWeakFormsRequirePhraseInterior(t *testing.T) {
	for _, tc := range []struct{ text, citation string }{
		{text: "of", citation: "AH1 V"},
		{text: "Of course.", citation: "AH1 V"},
		{text: "What is it made of?", citation: "AH1 V"},
		{text: "cup, of coffee", citation: "AH1 V"},
		{text: "cup of, coffee", citation: "AH1 V"},
		{text: "and then", citation: "AH0 N D"},
		{text: "than that", citation: "DH AE1 N"},
		{text: "some of us", citation: "S AH1 M"},
	} {
		got, _, err := englishPronunciation(tc.text, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, tc.citation) {
			t.Fatalf("%q: missing citation %q in %s", tc.text, tc.citation, got)
		}
	}
}

// ユーザー辞書と明示の読みは弱形より優先する。
func testEnglishWeakFormsPrecedence(t *testing.T) {
	got, _, err := englishPronunciation("the cat and the dog", "", map[string]string{"the": "DH IY1"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "DH IY1") || strings.Contains(got, "DH AH0") {
		t.Fatalf("dictionary override lost: %s", got)
	}
	reading := "DH AH0 | K AE1 T | AH0 N D | DH AH0 | D AO1 G"
	got, _, err = englishPronunciation("the cat and the dog", reading, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != reading {
		t.Fatalf("explicit reading changed: %s", got)
	}
}

// 大文字小文字は区別しない。既存のofと同じく大文字表記の機能語も弱形にし、
// テーブル外の語（固有名詞など）は引用形のまま。
func testEnglishWeakFormsCapitalization(t *testing.T) {
	got, _, err := englishPronunciation("A cup OF coffee.", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "AH0 V") {
		t.Fatalf("uppercase function word not weakened: %s", got)
	}
	got, _, err = englishPronunciation("NASA and the ISS", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "N AE1 S AH0") || !strings.Contains(got, "AH0 N") {
		t.Fatalf("proper noun or function word mishandled: %s", got)
	}
}

// 弱形を無効にすると引用形へ戻る。
func testEnglishWeakFormsDisabled(t *testing.T) {
	options := EnglishOptions{WeakForms: false}
	got, _, err := englishPronunciationWithOptions("bread and butter", "", nil, options)
	if err != nil {
		t.Fatal(err)
	}
	if got != "B R EH1 D | AH0 N D | B AH1 T ER0" {
		t.Fatalf("disabled weak form = %s", got)
	}
	for _, parse := range []func(string, string, map[string]string, EnglishOptions) (string, []Mora, error){
		ParseEnglishARPAsingWithOptions, ParseEnglishVCCVWithOptions, ParseEnglishCVWithOptions,
	} {
		reading, _, err := parse("bread and butter", "", nil, options)
		if err != nil {
			t.Fatal(err)
		}
		if reading != "B R EH1 D | AH0 N D | B AH1 T ER0" {
			t.Fatalf("disabled weak form = %s", reading)
		}
	}
	delta, _, err := ParseEnglishDeltaWithOptions("bread and butter", "", nil, PresampConfig{}, options)
	if err != nil {
		t.Fatal(err)
	}
	if delta != "B R EH1 D | AH0 N D | B AH1 T ER0" {
		t.Fatalf("delta disabled weak form = %s", delta)
	}
}
