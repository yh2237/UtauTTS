package frontend

import (
	"strings"
	"testing"
)

// 期待値はmisakiのJAG2P（cutlet）の出力。語の頭はOpenJTalkの解析から渡す。
func TestKokoroJapaneseSymbols(t *testing.T) {
	morae, err := ParseKana("キョーワイイテンキデスネ。")
	if err != nil {
		t.Fatal(err)
	}
	starts := make([]bool, len(morae))
	for _, i := range []int{0, 2, 3, 5, 8, 10} { // 今日/は/いい/天気/です/ね
		starts[i] = true
	}
	symbols, spaces := KokoroJapaneseSymbols(morae, starts)
	var b strings.Builder
	for i, symbol := range symbols {
		if spaces[i] {
			b.WriteString(" ")
		}
		b.WriteString(symbol)
	}
	if got := b.String(); got != "kʲoː βa iː teŋkʲi desɨ ne." {
		t.Fatalf("got %q", got)
	}
	// 語の区切りがなくても、っ・ん・小書きのかなは決まる。
	morae, _ = ParseKana("コンニチワ、ガッコーニキャク")
	symbols, _ = KokoroJapaneseSymbols(morae, nil)
	if got := strings.Join(symbols, ""); got != "koɲɲiʨiβa,ɡaʔkoːɲikʲakɯ" {
		t.Fatalf("got %q", got)
	}
}
