package frontend

import (
	"strings"
	"testing"
)

func TestToKanaUsesDictionaryPronunciation(t *testing.T) {
	got, err := ToKana("今日はいい天気です。")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "キョーワ") || !strings.HasSuffix(got, "デス。") {
		t.Fatalf("reading = %q", got)
	}
}

func TestToKanaPreservesKanaAndPunctuation(t *testing.T) {
	got, err := ToKana("こんにちは、テストです。")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "、") || !strings.HasSuffix(got, "。") {
		t.Fatalf("reading = %q", got)
	}
}

func TestToKanaIgnoresTokenWithoutPronunciation(t *testing.T) {
	got, err := ToKana("こんにちは🙂。")
	if err != nil {
		t.Fatal(err)
	}
	if got != "コンニチワ。" {
		t.Fatalf("reading = %q", got)
	}
}

func TestToKanaWithDictionaryOverridesSurfaceReading(t *testing.T) {
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

func TestApplyDictionaryPrefersLongestSurface(t *testing.T) {
	got := ApplyDictionary("東京都", map[string]string{
		"東京":  "とうきょう",
		"東京都": "とうきょうと",
	})
	if got != "とうきょうと" {
		t.Fatalf("replacement = %q", got)
	}
}

func TestToKanaWithDictionaryDoesNotReinterpretReading(t *testing.T) {
	got, err := ToKanaWithDictionary(" v8を使う。", map[string]string{"v8": "ぶいはち"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "ブイハチヲ") {
		t.Fatalf("reading = %q", got)
	}
}

func TestApplyDictionaryForAnalysisUsesKatakanaReading(t *testing.T) {
	got := ApplyDictionaryForAnalysis("v8を使う。", map[string]string{"v8": "ぶいはち"})
	if got != "ブイハチを使う。" {
		t.Fatalf("replacement = %q", got)
	}
}
