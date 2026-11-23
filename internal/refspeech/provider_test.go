package refspeech

import (
	"errors"
	"reflect"
	"testing"

	"github.com/yh2237/UtauTTS/internal/g2p/es"
	"github.com/yh2237/UtauTTS/internal/g2p/zh"
	"github.com/yh2237/UtauTTS/internal/linguistics"
)

func TestProviderRegistryAndCapabilities(t *testing.T) {
	for _, language := range []string{"en", "en-US", "zh-CN", "cmn", "es-ES", "pt-BR"} {
		if !SupportsProviderLanguage(language) {
			t.Errorf("SupportsProviderLanguage(%q) = false, want true", language)
		}
	}
	for _, language := range []string{"ja", "ja-JP", "unknown", ""} {
		if SupportsProviderLanguage(language) {
			t.Errorf("SupportsProviderLanguage(%q) = true, want false", language)
		}
	}
	for _, language := range []string{"zh-CN", "es-ES", "pt-BR"} {
		provider, err := NewProvider(language, ProviderConfig{})
		if err != nil || provider == nil {
			t.Errorf("%s: provider=%v, err=%v", language, provider, err)
		}
	}
	loaded := false
	provider, err := NewProvider("en-US", ProviderConfig{
		LoadLexicon: func() (*Lexicon, error) {
			loaded = true
			return nil, errors.New("load English lexicon: missing")
		},
	})
	if !loaded {
		t.Fatal("LoadLexicon was not called")
	}
	if err == nil || provider != nil {
		t.Fatalf("expected lexicon error, got provider=%v err=%v", provider, err)
	}
	if _, err := NewProvider("Unknown-XX", ProviderConfig{}); err == nil || err.Error() != `reference synthesis is not available for "Unknown-XX"` {
		t.Errorf("unsupported language: %v", err)
	}
}

func TestProviderAlignMoraTokens(t *testing.T) {
	pieces := []Piece{{Part: 0}, {Part: -1}, {Part: 1}}
	timing := Timing{PieceTokens: [][2]int{{1, 2}, {2, 3}, {3, 4}}, Durations: make([]int, 5)}
	utterance := linguistics.Utterance{
		Reading: "a b",
		Morae:   []linguistics.Mora{{Text: "a", WordIndex: 0}, {Text: "b", WordIndex: 1}},
	}
	for _, language := range []string{"en", "EN-us", "es", "es-ES", "pt", "pt-BR", "zh"} {
		provider, err := NewProvider(language, ProviderConfig{})
		if err != nil {
			t.Fatalf("%s: %v", language, err)
		}
		var want [][]int
		switch linguistics.NormalizeLanguage(language) {
		case linguistics.LanguageEnglish:
			want = englishMoraTokens(pieces, timing, utterance.EffectiveReading(), utterance.Morae)
		case linguistics.LanguageSpanish, linguistics.LanguagePortuguese:
			want = partMoraTokens(pieces, timing, utterance.Morae)
		default:
			want = simpleMoraTokens(pieces, timing, utterance.Morae)
		}
		if got := provider.AlignMoraTokens(pieces, timing, utterance); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %v, want %v", language, got, want)
		}
	}
}

func TestChineseProviderUsesSurfaceReading(t *testing.T) {
	utterance, err := zh.Parse("你好。", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if utterance.Reading != "ni3 hao3 |" || utterance.SurfaceReading != "ni2 hao3 |" {
		t.Fatalf("readings = %q / %q", utterance.Reading, utterance.SurfaceReading)
	}
	pieces, err := NewKokoroChineseProvider(nil).Pieces(utterance)
	if err != nil {
		t.Fatal(err)
	}
	if len(pieces) != 3 || pieces[0].Symbols != "ㄋㄧ2" || pieces[1].Symbols != "ㄏㄠ3" || pieces[2].Symbols != "," {
		t.Fatalf("pieces = %+v", pieces)
	}
}

func TestSyllableProviderPiecesAndAlign(t *testing.T) {
	utterance, err := es.Parse("", "ˈo.la , kˈe tˈal ?", nil)
	if err != nil {
		t.Fatal(err)
	}
	provider := NewKokoroSyllableProvider(nil, linguistics.LanguageSpanish)
	pieces, err := provider.Pieces(utterance)
	if err != nil {
		t.Fatal(err)
	}
	if len(pieces) != 8 || pieces[0].Part != 0 || pieces[2].Symbols != "," || pieces[2].Part != 2 || pieces[7].Symbols != "?" || pieces[7].Part != -1 {
		t.Fatalf("pieces = %+v", pieces)
	}
}
