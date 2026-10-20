package tts

import (
	"testing"

	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/refspeech"
)

// TestSimpleMoraTokensは、1音節が1モーラになる言語で、記号とモーラが1対1になることを確かめる。
func TestSimpleMoraTokens(t *testing.T) {
	reading, morae, err := frontend.ParseChineseCVVC("你好，今天。", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	pieces, err := refspeech.NewKokoroChineseProvider(nil).Pieces("你好，今天。", reading)
	if err != nil {
		t.Fatal(err)
	}
	// 音節と休止の数が、記号とモーラの数に一致する。
	if len(pieces) != len(morae) {
		t.Fatalf("%d pieces for %d morae (reading %q)", len(pieces), len(morae), reading)
	}
	timing := refspeech.Timing{Durations: make([]int, len(pieces)+2), PieceTokens: make([][2]int, len(pieces))}
	for i := range timing.PieceTokens {
		timing.PieceTokens[i] = [2]int{i + 1, i + 2}
	}
	tokens := simpleMoraTokens(pieces, timing, morae)
	if len(tokens) != len(morae) {
		t.Fatalf("%d token lists for %d morae", len(tokens), len(morae))
	}
	for i := range morae {
		if len(tokens[i]) == 0 {
			t.Fatalf("mora %d has no tokens", i)
		}
	}
	// 末尾の境界記号は最後のモーラに入る。
	last := tokens[len(tokens)-1]
	if last[len(last)-1] != len(timing.Durations)-1 {
		t.Fatalf("last mora does not end with the final boundary token: %v", last)
	}
}

// TestSimpleMoraTokensMismatchは、数が合わないときに空を返し、誤った割り当てをしないことを確かめる。
func TestSimpleMoraTokensMismatch(t *testing.T) {
	pieces := []refspeech.Piece{{Symbols: "a", Part: 0}}
	morae := []frontend.Mora{{Text: "a"}, {Text: "i"}}
	tokens := simpleMoraTokens(pieces, refspeech.Timing{}, morae)
	if len(tokens) != len(morae) {
		t.Fatalf("got %d token lists, want %d", len(tokens), len(morae))
	}
	for i, list := range tokens {
		if len(list) != 0 {
			t.Fatalf("mora %d got tokens despite the mismatch: %v", i, list)
		}
	}
}

// TestReferenceProvidersは、モデルの有無で一覧が変わることを確かめる。
func TestReferenceProviders(t *testing.T) {
	if providers := ReferenceProviders(""); providers != nil {
		t.Fatalf("providers for an empty folder: %v", providers)
	}
	if providers := ReferenceProviders(t.TempDir()); providers != nil {
		t.Fatalf("providers for a folder without models: %v", providers)
	}
}
