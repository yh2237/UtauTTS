package refspeech

import (
	"strings"
	"testing"
)

func TestPinyinToZhuyin(t *testing.T) {
	for _, test := range []struct {
		pinyin, want string
	}{
		{"ni3", "ㄋㄧ3"},
		{"hao3", "ㄏㄠ3"},
		// 一部の韻母は、注音符号ではなく1文字で書く（misakiの表と同じ）。
		{"zhong1", "ㄓ中1"},
		{"guo2", "ㄍ我2"},
		{"ren2", "ㄖㄣ2"},
		{"jin1", "ㄐ阴1"},
		{"tian1", "ㄊ言1"},
		{"qi4", "ㄑㄧ4"},
		// zi・siとzhi・shiのiは、それぞれ違う韻母として書く。
		{"zi3", "ㄗㄭ3"},
		{"si1", "ㄙㄭ1"},
		{"zhi1", "ㄓ十1"},
		{"shi2", "ㄕ十2"},
		// y・wで始まる綴りとüの綴りは、声母と韻母の組に直す。
		{"yi1", "ㄧ1"},
		{"ye4", "ㄝ4"},
		{"wu3", "ㄨ3"},
		{"wo3", "我3"},
		{"yue4", "月4"},
		{"lv4", "ㄌㄩ4"},
		{"ju1", "ㄐㄩ1"},
		{"jun1", "ㄐ云1"},
		{"er2", "ㄦ2"},
		{"iu4", "又4"},
		{"de5", "ㄉㄜ5"},
	} {
		got, ok := pinyinToZhuyin(test.pinyin)
		if !ok {
			t.Errorf("pinyinToZhuyin(%q) failed", test.pinyin)
			continue
		}
		if got != test.want {
			t.Errorf("pinyinToZhuyin(%q) = %q, want %q", test.pinyin, got, test.want)
		}
	}
}

func TestZhuyinPieces(t *testing.T) {
	provider := NewKokoroChineseProvider(nil)
	pieces, err := provider.Pieces("", "ni3 hao3 | jin1 tian1 tian1 qi4 |")
	if err != nil {
		t.Fatal(err)
	}
	var symbols []string
	for _, piece := range pieces {
		symbols = append(symbols, piece.Symbols)
	}
	got := strings.Join(symbols, " ")
	want := "ㄋㄧ3 ㄏㄠ3 , ㄐ阴1 ㄊ言1 ㄊ言1 ㄑㄧ4 ,"
	if got != want {
		t.Fatalf("Pieces = %q, want %q", got, want)
	}
	// 休止の断片だけが語の番号を持たない。
	if pieces[2].Part != -1 || pieces[0].Part != 0 || pieces[1].Part != 1 || pieces[3].Part != 2 {
		t.Fatalf("part numbering is wrong: %+v", pieces)
	}
}

func TestZhuyinPiecesNeedReading(t *testing.T) {
	provider := NewKokoroChineseProvider(nil)
	if _, err := provider.Pieces("你好", ""); err == nil {
		t.Fatal("expected an error without a reading")
	}
	if _, err := provider.Pieces("", "ni3 nope"); err == nil {
		t.Fatal("expected an error for an unknown syllable")
	}
}

func TestChineseProviderIdentity(t *testing.T) {
	provider := NewKokoroChineseProvider(nil)
	if provider.ID() != "kokoro-zh" || provider.Language() != "zh" {
		t.Fatalf("unexpected identity: %q %q", provider.ID(), provider.Language())
	}
}
