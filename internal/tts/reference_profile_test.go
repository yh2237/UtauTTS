//go:build !js

package tts

import (
	"reflect"
	"strings"
	"testing"

	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/refspeech"
)

func TestReferenceProfileReading(t *testing.T) {
	for _, text := range []string{"你好。", "很好。", "我们。", "不是。", "一个。", "一天。"} {
		reading, morae, err := frontend.ParseChineseCVVC(text, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, language := range []string{"zh", "zh-CN", "cmn"} {
			if got, want := referenceProfileFor(language).reading(reading, morae), chineseSurfaceReading(reading, morae); got != want {
				t.Errorf("%s %q: got %q, want %q", language, text, got, want)
			}
		}
		for _, language := range []string{"en", "es", "pt", "ja", "unknown"} {
			if got := referenceProfileFor(language).reading(reading, morae); got != reading {
				t.Errorf("%s: reading changed to %q", language, got)
			}
		}
	}
}

func TestReferenceProfileMoraTokens(t *testing.T) {
	pieces := []refspeech.Piece{{Part: 0}, {Part: -1}, {Part: 1}}
	timing := refspeech.Timing{PieceTokens: [][2]int{{1, 2}, {2, 3}, {3, 4}}, Durations: make([]int, 5)}
	morae := []frontend.Mora{{Text: "a", WordIndex: 0}, {Text: "b", WordIndex: 1}}
	const reading = "a b"
	for _, language := range []string{"en", "EN-us", "es", "es-ES", "pt", "pt-BR", "zh", "ja", "unknown", ""} {
		var want [][]int
		switch frontend.NormalizeLanguage(language) {
		case frontend.LanguageEnglish:
			want = englishMoraTokens(pieces, timing, reading, morae)
		case frontend.LanguageSpanish, frontend.LanguagePortuguese:
			want = partMoraTokens(pieces, timing, morae)
		default:
			want = simpleMoraTokens(pieces, timing, morae)
		}
		if got := referenceMoraTokens(language, pieces, timing, reading, morae); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %v, want %v", language, got, want)
		}
	}
}

func TestReferenceProfileProvider(t *testing.T) {
	for _, language := range []string{"zh-CN", "es-ES", "pt-BR"} {
		provider, err := referenceProfileFor(language).provider(Config{}, nil)
		if err != nil || provider == nil {
			t.Errorf("%s: provider=%v, err=%v", language, provider, err)
		}
	}
	profile := referenceProfileFor("en-US")
	cfg := Config{ReferenceDir: t.TempDir()}
	_, gold, _ := ReferenceModels(cfg.ReferenceDir)
	if _, loaded := englishLexicons.Load(gold); loaded {
		t.Fatal("lexicon loaded before provider creation")
	}
	if _, err := profile.provider(cfg, nil); err == nil || !strings.Contains(err.Error(), "load English lexicon:") {
		t.Errorf("missing lexicon: %v", err)
	}
	const language = "Unknown-XX"
	if _, err := referenceProfileFor(language).provider(Config{}, nil); err == nil || err.Error() != `reference synthesis is not available for "Unknown-XX"` {
		t.Errorf("unsupported language: %v", err)
	}
}
