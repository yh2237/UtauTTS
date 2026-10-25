package g2p

import (
	"strings"
	"testing"
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
		if got := Normalize(Portuguese, input); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestLookupEnglishSources(t *testing.T) {
	tests := map[string]Source{
		"google":   SourceCMU,
		"untended": SourceLibriSpeech,
	}
	for word, want := range tests {
		entry, err := LookupEnglish(word)
		if err != nil {
			t.Fatalf("%s: %v", word, err)
		}
		if entry.Phones == "" || entry.Source != want {
			t.Errorf("%s: entry = %+v, want source %q", word, entry, want)
		}
	}
}

func TestReadingCache(t *testing.T) {
	cache := NewReadingCache(2)
	cache.Put("a", Reading{Phones: "AH0", Source: SourceCMU})
	if reading, ok := cache.Get("a"); !ok || reading.Phones != "AH0" {
		t.Fatalf("cache miss: %+v %v", reading, ok)
	}
	cache.Put("b", Reading{Phones: "B IY1"})
	cache.Put("c", Reading{Phones: "S IY1"})
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

func TestEnglishNumberWords(t *testing.T) {
	tests := map[string]string{
		"21":      "twenty one",
		"2024":    "two thousand twenty four",
		"3.50":    "three point five zero",
		"007":     "zero zero seven",
		"1000000": "one million",
	}
	for input, want := range tests {
		if got := EnglishNumber(input); got != want {
			t.Errorf("EnglishNumber(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSpanishNumberWords(t *testing.T) {
	tests := map[string]string{
		"21":      "veintiuno",
		"35":      "treinta y cinco",
		"2024":    "dos mil veinticuatro",
		"21000":   "veintiún mil",
		"1000000": "un millón",
		"3500000": "tres millones quinientos mil",
	}
	for input, want := range tests {
		if got := SpanishNumberWords(input); got != want {
			t.Errorf("SpanishNumberWords(%q) = %q, want %q", input, got, want)
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
		if got := Normalize(Spanish, input); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestPortugueseVeryLargeNumberFallsBackToDigits(t *testing.T) {
	if got := Normalize(Portuguese, "1.000.000.000.000"); !strings.HasPrefix(got, "um zero") {
		t.Fatalf("Normalize very large number = %q", got)
	}
}

func TestForeignReading(t *testing.T) {
	tests := []struct {
		language Language
		word     string
		want     string
	}{
		{Spanish, "OpenAI", "ˌo.pen.ˈaj"},
		{Spanish, "Google", "ˈɡu.ɣel"},
		{Portuguese, "ChatGPT", "ʃˈat.ʒe.pe.tˈe"},
		{Portuguese, "LinkedIn", "lin.ke.ˈdʒĩ"},
		{Spanish, "Instagram", "ˌins.ta.ɡɾˈam"},
		{Chinese, "OpenAI", "ou1 pen1 ei1 ai4"},
		{Chinese, "iPhone", "ai4 feng1"},
		{Chinese, "Spotify", "si1 bo1 ti1 fei1"},
		{English, "DeepSeek", "D IY1 P S IY2 K"},
		{English, "Kubernetes", "K UW2 B ER0 N EH1 T IY0 Z"},
	}
	for _, test := range tests {
		got, ok := ForeignReading(test.language, test.word)
		if !ok || got != test.want {
			t.Errorf("ForeignReading(%s, %q) = %q, %v; want %q", test.language, test.word, got, ok, test.want)
		}
	}
}

func TestChineseLatinReading(t *testing.T) {
	tests := map[string]string{
		"AI":  "ei1 ai4",
		"GDP": "ji1 di1 pi1",
		"T":   "ti1",
	}
	for input, want := range tests {
		if got := ChineseLatinReading(input); got != want {
			t.Errorf("ChineseLatinReading(%q) = %q, want %q", input, got, want)
		}
	}
}
