package g2p_test

import (
	"strings"
	"testing"

	"github.com/yh2237/UtauTTS/internal/g2p"
	_ "github.com/yh2237/UtauTTS/internal/g2p/all"
	"github.com/yh2237/UtauTTS/internal/linguistics"
)

func TestNormalizePortugueseNumbers(t *testing.T) {
	tests := map[string]string{
		"2026":            "dois mil e vinte e seis",
		"1.234":           "mil duzentos e trinta e quatro",
		"1.200":           "mil e duzentos",
		"1.000.000":       "um milhão",
		"3,50":            "três vírgula cinquenta",
		"R$ 3,50":         "três reais e cinquenta centavos",
		"R$ 1,00":         "um real",
		"R$ 0,50":         "cinquenta centavos",
		"US$ 3,50":        "três dólares e cinquenta centavos",
		"€ 2,00":          "dois euros",
		"50%":             "cinquenta por cento",
		"O preço é 3,50.": "O preço é três vírgula cinquenta.",
	}
	for input, want := range tests {
		if got := g2p.Normalize(g2p.Portuguese, input); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeSpanishCurrencyAndPercent(t *testing.T) {
	tests := map[string]string{
		"El precio es 3,50 €.": "El precio es 3 euros con 50 céntimos.",
		"€ 3,50":               "3 euros con 50 céntimos",
		"1 €":                  "1 euro",
		"0,50 €":               "50 céntimos",
		"US$ 3,50":             "3 dólares con 50 centavos",
		"3,50":                 "3 coma 50",
		"50% de descuento":     "50 por ciento de descuento",
	}
	for input, want := range tests {
		if got := g2p.Normalize(g2p.Spanish, input); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestPortugueseVeryLargeNumberFallsBackToDigits(t *testing.T) {
	if got := g2p.Normalize(g2p.Portuguese, "1.000.000.000.000"); !strings.HasPrefix(got, "um zero") {
		t.Fatalf("Normalize very large number = %q", got)
	}
}

func TestReadingCache(t *testing.T) {
	cache := g2p.NewReadingCache(2)
	cache.Put("a", g2p.Reading{Phones: "AH0", Source: g2p.SourceCMU})
	if reading, ok := cache.Get("a"); !ok || reading.Phones != "AH0" {
		t.Fatalf("cache miss: %+v %v", reading, ok)
	}
	cache.Put("b", g2p.Reading{Phones: "B IY1"})
	cache.Put("c", g2p.Reading{Phones: "S IY1"})
	present := 0
	for _, key := range []string{"a", "b", "c"} {
		if _, ok := cache.Get(key); ok {
			present++
		}
	}
	if present != 2 {
		t.Fatalf("cache holds %d entries, want 2", present)
	}
}

func TestForeignReading(t *testing.T) {
	tests := []struct {
		language g2p.Language
		word     string
		want     string
	}{
		{g2p.Spanish, "OpenAI", "ˌo.pen.ˈaj"},
		{g2p.Spanish, "Google", "ˈɡu.ɣel"},
		{g2p.Portuguese, "ChatGPT", "ʃˈat.ʒe.pe.tˈe"},
		{g2p.Portuguese, "LinkedIn", "lin.ke.ˈdʒĩ"},
		{g2p.Spanish, "Instagram", "ˌins.ta.ɡɾˈam"},
		{g2p.Chinese, "OpenAI", "ou1 pen1 ei1 ai4"},
		{g2p.Chinese, "iPhone", "ai4 feng1"},
		{g2p.Chinese, "Spotify", "si1 bo1 ti1 fei1"},
		{g2p.English, "DeepSeek", "D IY1 P S IY2 K"},
		{g2p.English, "Kubernetes", "K UW2 B ER0 N EH1 T IY0 Z"},
	}
	for _, test := range tests {
		got, ok := g2p.ForeignReading(test.language, test.word)
		if !ok || got != test.want {
			t.Errorf("ForeignReading(%s, %q) = %q, %v; want %q", test.language, test.word, got, ok, test.want)
		}
	}
}

func TestResolveLanguageDefaultsAndPhonemizers(t *testing.T) {
	language, phonemizer, err := g2p.ResolveLanguage("zh", "")
	if err != nil || language != linguistics.LanguageChinese || phonemizer != linguistics.PhonemizerChinese {
		t.Fatalf("language=%q phonemizer=%q err=%v", language, phonemizer, err)
	}
	for _, p := range []string{
		linguistics.PhonemizerEnglish,
		linguistics.PhonemizerEnglishDelta,
		linguistics.PhonemizerEnglishVCCV,
		linguistics.PhonemizerEnglishCV,
	} {
		lang, resolved, err := g2p.ResolveLanguage("en", p)
		if err != nil || lang != linguistics.LanguageEnglish || resolved != p {
			t.Fatalf("phonemizer=%q lang=%q resolved=%q err=%v", p, lang, resolved, err)
		}
	}
	lang, resolved, err := g2p.ResolveLanguage("", linguistics.PhonemizerEnglishDelta)
	if err != nil || lang != linguistics.LanguageEnglish || resolved != linguistics.PhonemizerEnglishDelta {
		t.Fatalf("inferred lang=%q resolved=%q err=%v", lang, resolved, err)
	}
	if _, _, err := g2p.ResolveLanguage("fr", ""); err == nil {
		t.Fatal("unsupported language accepted")
	}
	if _, _, err := g2p.ResolveLanguage("ja", linguistics.PhonemizerEnglish); err == nil {
		t.Fatal("mismatched phonemizer accepted")
	}
}
