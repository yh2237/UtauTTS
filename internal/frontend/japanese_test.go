package frontend

import (
	"strings"
	"testing"
)

func TestToKana(t *testing.T) {
	t.Run("dictionary pronunciation", testToKanaUsesDictionaryPronunciation)
	t.Run("preserves kana and punctuation", testToKanaPreservesKanaAndPunctuation)
	t.Run("ignores token without pronunciation", testToKanaIgnoresTokenWithoutPronunciation)
}

func TestToKanaWithDictionary(t *testing.T) {
	t.Run("overrides surface reading", testToKanaWithDictionaryOverridesSurfaceReading)
	t.Run("prefers longest surface", testApplyDictionaryPrefersLongestSurface)
	t.Run("does not reinterpret reading", testToKanaWithDictionaryDoesNotReinterpretReading)
	t.Run("analysis uses katakana reading", testApplyDictionaryForAnalysisUsesKatakanaReading)
}

func testToKanaUsesDictionaryPronunciation(t *testing.T) {
	got, err := ToKana("今日はいい天気です。")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "キョーワ") || !strings.HasSuffix(got, "デス。") {
		t.Fatalf("reading = %q", got)
	}
}

func testToKanaPreservesKanaAndPunctuation(t *testing.T) {
	got, err := ToKana("こんにちは、テストです。")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "、") || !strings.HasSuffix(got, "。") {
		t.Fatalf("reading = %q", got)
	}
}

func testToKanaIgnoresTokenWithoutPronunciation(t *testing.T) {
	got, err := ToKana("こんにちは🙂。")
	if err != nil {
		t.Fatal(err)
	}
	if got != "コンニチワ。" {
		t.Fatalf("reading = %q", got)
	}
}

func testToKanaWithDictionaryOverridesSurfaceReading(t *testing.T) {
	got, err := ToKanaWithDictionary("UtauTTSを試します。", map[string]string{
		"UtauTTS": "うたうてぃーてぃーえす",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "ウタウテ") {
		t.Fatalf("reading = %q", got)
	}
}

func testApplyDictionaryPrefersLongestSurface(t *testing.T) {
	got := ApplyDictionary("東京都", map[string]string{
		"東京":  "とうきょう",
		"東京都": "とうきょうと",
	})
	if got != "とうきょうと" {
		t.Fatalf("replacement = %q", got)
	}
}

func testToKanaWithDictionaryDoesNotReinterpretReading(t *testing.T) {
	got, err := ToKanaWithDictionary(" v8を使う。", map[string]string{"v8": "ぶいはち"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "ブイハチヲ") {
		t.Fatalf("reading = %q", got)
	}
}

func testApplyDictionaryForAnalysisUsesKatakanaReading(t *testing.T) {
	got := ApplyDictionaryForAnalysis("v8を使う。", map[string]string{"v8": "ぶいはち"})
	if got != "ブイハチを使う。" {
		t.Fatalf("replacement = %q", got)
	}
}
