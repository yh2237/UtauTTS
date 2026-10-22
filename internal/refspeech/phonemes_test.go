package refspeech

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKokoroPhonemesFromReading(t *testing.T) {
	for _, test := range []struct {
		text, reading, want string
	}{
		{"Hello, my name is Sam.", "HH AH0 L OW1 | SP | M AY1 | N EY1 M | IH1 Z | S AE1 M | SP", "həlˈO, mˈI nˈAm ˈɪz sˈæm."},
		{"What time does the meeting start?", "W AH1 T | T AY1 M | D AH0 Z | DH AH0 | M IY1 T IH0 NG | S T AA1 R T | SP", "wˈʌt tˈIm dəz ðə mˈitɪŋ stˈɑɹt?"},
	} {
		if got := joinPieces(KokoroPieces(nil, test.text, test.reading, test.reading, nil)); got != test.want {
			t.Errorf("KokoroPieces(%q) = %q, want %q", test.reading, got, test.want)
		}
	}
}

// REFSPEECH_LEXICONにmisakiのus_gold.jsonとus_silver.jsonがあるフォルダを指定すると、misakiの出力と比べる。
func TestKokoroPhonemesMatchMisaki(t *testing.T) {
	dir := os.Getenv("REFSPEECH_LEXICON")
	if dir == "" {
		t.Skip("set REFSPEECH_LEXICON")
	}
	lexicon, err := LoadLexicon(filepath.Join(dir, "us_gold.json"), filepath.Join(dir, "us_silver.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		text, reading, want string
	}{
		{"Hello, my name is Sam.", "HH AH0 L OW1 | SP | M AY1 | N EY1 M | IH1 Z | S AE1 M | SP", "həlˈO, mI nˈAm ɪz sˈæm."},
		{"The weather is really nice today.", "DH AH0 | W EH1 DH ER0 | IH1 Z | R IH1 L IY0 | N AY1 S | T AH0 D EY1 | SP", "ðə wˈɛðəɹ ɪz ɹˈiᵊli nˈIs tədˈA."},
		{"Could you help me with this box?", "K UH1 D | Y UW1 | HH EH1 L P | M IY1 | W IH1 DH | DH IH1 S | B AA1 K S | SP", "kˌʊd ju hˈɛlp mˌi wɪð ðɪs bˈɑks?"},
		{"I had chocolate cake for breakfast.", "AY1 | HH AE1 D | CH AO1 K L AH0 T | K EY1 K | F ER0 | B R EH1 K F AH0 S T | SP", "ˌI hæd ʧˈɑkᵊlət kˈAk fɔɹ bɹˈɛkfəst."},
		{"Please send me the message tomorrow.", "P L IY1 Z | S EH1 N D | M IY1 | DH AH0 | M EH1 S AH0 JH | T AH0 M AA1 R OW2 | SP", "plˈiz sˈɛnd mˌi ðə mˈɛsɪʤ təmˈɑɹO."},
		{"What time does the meeting start?", "W AH1 T | T AY1 M | D AH0 Z | DH AH0 | M IY1 T IH0 NG | S T AA1 R T | SP", "wˌʌt tˈIm dˈʌz ðə mˈiTɪŋ stˈɑɹt?"},
		{"Let's take a short break.", "L EH1 T S | T EY1 K | AH0 | SH AO1 R T | B R EY1 K | SP", "lˈɛts tˈAk ɐ ʃˈɔɹt bɹˈAk."},
	} {
		if got := joinPieces(KokoroPieces(lexicon, test.text, test.reading, test.reading, nil)); got != test.want {
			t.Errorf("KokoroPieces(%q) = %q, want %q", test.text, got, test.want)
		}
	}
}

// 自動の読みから直した語だけを、misakiの辞書ではなく直した読みから作る。
func TestKokoroPhonemesUseEditedWords(t *testing.T) {
	dir := os.Getenv("REFSPEECH_LEXICON")
	if dir == "" {
		t.Skip("set REFSPEECH_LEXICON")
	}
	lexicon, err := LoadLexicon(filepath.Join(dir, "us_gold.json"), filepath.Join(dir, "us_silver.json"))
	if err != nil {
		t.Fatal(err)
	}
	automatic := "HH AH0 L OW1 | SP | M AY1 | N EY1 M | IH1 Z | S AE1 M | SP"
	for _, test := range []struct {
		reading, want string
	}{
		{automatic, "həlˈO, mI nˈAm ɪz sˈæm."},
		{"HH AH0 L OW1 | SP | M AY1 | N EY1 M | IH1 Z | S EY1 M | SP", "həlˈO, mI nˈAm ɪz sˈAm."},
		{"B AY1 | SP", "bˈI."},
	} {
		if got := joinPieces(KokoroPieces(lexicon, "Hello, my name is Sam.", test.reading, automatic, nil)); got != test.want {
			t.Errorf("KokoroPieces(%q) = %q, want %q", test.reading, got, test.want)
		}
	}
}

// joinPiecesは断片をつないで、Kokoroに渡す音素列にする。
func joinPieces(pieces []Piece) string {
	var b strings.Builder
	for _, piece := range pieces {
		b.WriteString(piece.Symbols)
	}
	return b.String()
}
