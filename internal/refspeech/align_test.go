package refspeech

import (
	"reflect"
	"testing"

	"github.com/yh2237/UtauTTS/internal/g2p/zh"
	"github.com/yh2237/UtauTTS/internal/linguistics"
)

func TestAlignEnglishWordFollowsPhones(t *testing.T) {
	phones := func(symbols ...string) []linguistics.Phone {
		out := make([]linguistics.Phone, len(symbols))
		for i, s := range symbols {
			out[i] = linguistics.Phone{Symbol: s}
		}
		return out
	}
	for _, test := range []struct {
		symbols []string
		phones  []linguistics.Phone
		want    []int
	}{
		// hello: həlˈO
		{[]string{"h", "ə", "l", "ˈ", "O"}, phones("hh", "ah", "l", "ow"), []int{0, 1, 2, 3, 3}},
		// weather: wˈɛðəɹ
		{[]string{"w", "ˈ", "ɛ", "ð", "ə", "ɹ"}, phones("w", "eh", "dh", "er"), []int{0, 1, 1, 2, 3, 3}},
		// today: tədˈA
		{[]string{"t", "ə", "d", "ˈ", "A"}, phones("t", "ah", "d", "ey"), []int{0, 1, 2, 3, 3}},
		// really: ɹˈiᵊli
		{[]string{"ɹ", "ˈ", "i", "ᵊ", "l", "i"}, phones("r", "ih", "l", "iy"), []int{0, 1, 1, 1, 2, 3}},
	} {
		if got := alignEnglishWord(test.symbols, test.phones); !reflect.DeepEqual(got, test.want) {
			t.Errorf("alignEnglishWord(%q) = %v, want %v", test.symbols, got, test.want)
		}
	}
}

func TestEnglishMoraTokens(t *testing.T) {
	utterance := linguistics.Utterance{
		Language: linguistics.LanguageEnglish,
		Reading:  "HH AH0 L OW1 | SP | W ER1 L D | SP",
	}
	pieces := []Piece{
		{Symbols: "həlˈO", Part: 0}, {Symbols: ",", Part: 1}, {Symbols: " ", Part: -1},
		{Symbols: "wˈɜɹld", Part: 2}, {Symbols: ".", Part: -1},
	}
	durations := make([]int, 16)
	for i := range durations {
		durations[i] = 2
	}
	timing := Timing{Durations: durations, PieceTokens: [][2]int{{1, 6}, {6, 7}, {7, 8}, {8, 14}, {14, 15}}}
	two := []linguistics.Phone{{Symbol: "a"}, {Symbol: "b"}}
	utterance.Morae = []linguistics.Mora{
		{WordIndex: 0, Phones: two}, {WordIndex: 0, Phones: two}, {Pause: true},
		{WordIndex: 2, Phones: append(two, two...)}, {Pause: true},
	}
	provider := NewKokoroProvider(nil, nil, nil)
	tokens := provider.AlignMoraTokens(pieces, timing, utterance)
	if !reflect.DeepEqual(tokens, [][]int{{1, 2, 3}, {4, 5}, {6, 7}, {8, 9, 10, 11, 12, 13}, {14, 15}}) {
		t.Fatalf("tokens = %v", tokens)
	}
}

// TestSimpleMoraTokensは、1音節が1モーラになる言語で、記号とモーラが1対1になることを確かめる。
func TestSimpleMoraTokens(t *testing.T) {
	utterance, err := zh.Parse("你好，今天。", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	pieces, err := NewKokoroChineseProvider(nil).Pieces(utterance)
	if err != nil {
		t.Fatal(err)
	}
	// 音節と休止の数が、記号とモーラの数に一致する。
	if len(pieces) != len(utterance.Morae) {
		t.Fatalf("%d pieces for %d morae (reading %q)", len(pieces), len(utterance.Morae), utterance.EffectiveReading())
	}
	timing := Timing{Durations: make([]int, len(pieces)+2), PieceTokens: make([][2]int, len(pieces))}
	for i := range timing.PieceTokens {
		timing.PieceTokens[i] = [2]int{i + 1, i + 2}
	}
	tokens := simpleMoraTokens(pieces, timing, utterance.Morae)
	if len(tokens) != len(utterance.Morae) {
		t.Fatalf("%d token lists for %d morae", len(tokens), len(utterance.Morae))
	}
	for i := range utterance.Morae {
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
	pieces := []Piece{{Symbols: "a", Part: 0}}
	morae := []linguistics.Mora{{Text: "a"}, {Text: "i"}}
	tokens := simpleMoraTokens(pieces, Timing{}, morae)
	if len(tokens) != len(morae) {
		t.Fatalf("got %d token lists, want %d", len(tokens), len(morae))
	}
	for i, list := range tokens {
		if len(list) != 0 {
			t.Fatalf("mora %d got tokens despite the mismatch: %v", i, list)
		}
	}
}
