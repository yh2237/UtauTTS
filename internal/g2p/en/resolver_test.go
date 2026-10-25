package en

import (
	"testing"

	"github.com/yh2237/UtauTTS/internal/g2p"
)

func TestResolveWithContextPrefersAcronyms(t *testing.T) {
	reading, err := ResolveWithContext("AI", func(string) (string, bool) {
		return "ˈaɪ", true
	})
	if err != nil {
		t.Fatal(err)
	}
	if reading.Source != g2p.SourceAcronym || reading.Notation == g2p.NotationKokoro {
		t.Fatalf("AI = %+v, want acronym ARPAbet", reading)
	}
}

func TestResolveWithContextUsesContextThenFallsBack(t *testing.T) {
	reading, err := ResolveWithContext("google", func(string) (string, bool) {
		return "ɡˈuɡᵊl", true
	})
	if err != nil {
		t.Fatal(err)
	}
	if reading.Source != g2p.SourceMisaki || reading.Notation != g2p.NotationKokoro {
		t.Fatalf("google = %+v, want misaki Kokoro", reading)
	}
	reading, err = ResolveWithContext("google", func(string) (string, bool) {
		return "", false
	})
	if err != nil {
		t.Fatal(err)
	}
	if reading.Source != g2p.SourceCMU {
		t.Fatalf("google fallback = %+v, want CMU", reading)
	}
}

func TestEnglishResolverSources(t *testing.T) {
	tests := map[string]g2p.Source{
		"ChatGPT":  g2p.SourceAcronym,
		"OpenAI":   g2p.SourceAcronym,
		"google":   g2p.SourceCMU,
		"untended": g2p.SourceLibriSpeech,
		"Miku":     g2p.SourceCompound,
		"Zundamon": g2p.SourceEntity,
		"DeepSeek": g2p.SourceEntity,
		"phlame":   g2p.SourceRules,
	}
	for word, want := range tests {
		reading, err := englishResolve(word)
		if err != nil {
			t.Fatalf("%s: %v", word, err)
		}
		if reading.Source != want {
			t.Errorf("%s: source = %q, want %q (reading %q)", word, reading.Source, want, reading.Phones)
		}
	}
}
