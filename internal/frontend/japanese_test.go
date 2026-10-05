package frontend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ikawaha/kagome-dict/ipa"
	"github.com/ikawaha/kagome/v2/tokenizer"
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

func TestToKanaMatchesFullDictionaryReadings(t *testing.T) {
	texts := []string{
		"東京特許許可局で、2026年10月1日に会議を行います。",
		"ＵＴＡＵとTTSを組み合わせた音声合成！",
		"彼は「明日行けたら行く」と言った。",
		"ｶﾀｶﾅ、ひらがな、漢字、記号☆を混ぜた文章です。",
		"未知語のゑゐやヴァイオリン、ﾄﾞｰﾅﾂも読む。",
		"🎉おめでとうございます🎉",
	}
	files, err := filepath.Glob(filepath.Join("..", "..", "tools", "evaluation", "japanese-*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("evaluation corpora: %v", err)
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var items []struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(data, &items); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		for _, item := range items {
			texts = append(texts, item.Text)
		}
	}
	full, err := tokenizer.New(ipa.Dict(), tokenizer.OmitBosEos())
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range texts {
		got, gotErr := ToKana(text)
		want, wantErr := readTokens(full.Tokenize(strings.TrimSpace(text)), nil)
		if got != want || (gotErr == nil) != (wantErr == nil) {
			t.Fatalf("%q: got %q/%v want %q/%v", text, got, gotErr, want, wantErr)
		}
	}
	if japanesePronunciations == nil {
		t.Fatal("reading conversion fell back to the full dictionary")
	}
}
