//go:build !js

package tts

import (
	"fmt"

	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/refspeech"
)

type referenceMatchingProfile struct {
	provider   func(Config, *refspeech.Service) (refspeech.Provider, error)
	reading    func(string, []frontend.Mora) string
	moraTokens func([]refspeech.Piece, refspeech.Timing, string, []frontend.Mora) [][]int
}

func referenceProfileFor(language string) referenceMatchingProfile {
	profile := referenceMatchingProfile{
		provider: func(Config, *refspeech.Service) (refspeech.Provider, error) {
			return nil, fmt.Errorf("reference synthesis is not available for %q", language)
		},
		reading: func(reading string, _ []frontend.Mora) string { return reading },
		moraTokens: func(pieces []refspeech.Piece, timing refspeech.Timing, _ string, morae []frontend.Mora) [][]int {
			return simpleMoraTokens(pieces, timing, morae)
		},
	}
	switch normalized := frontend.NormalizeLanguage(language); normalized {
	case frontend.LanguageChinese:
		profile.provider = func(_ Config, service *refspeech.Service) (refspeech.Provider, error) {
			return refspeech.NewKokoroChineseProvider(service), nil
		}
		profile.reading = chineseSurfaceReading
	case frontend.LanguageSpanish, frontend.LanguagePortuguese:
		profile.provider = func(_ Config, service *refspeech.Service) (refspeech.Provider, error) {
			return refspeech.NewKokoroSyllableProvider(service, normalized), nil
		}
		profile.moraTokens = func(pieces []refspeech.Piece, timing refspeech.Timing, _ string, morae []frontend.Mora) [][]int {
			return partMoraTokens(pieces, timing, morae)
		}
	case frontend.LanguageEnglish:
		profile.provider = func(cfg Config, service *refspeech.Service) (refspeech.Provider, error) {
			lexicon, err := englishLexicon(cfg.ReferenceDir)
			if err != nil {
				return nil, err
			}
			return refspeech.NewKokoroProvider(service, lexicon, cfg.Dictionary), nil
		}
		profile.moraTokens = englishMoraTokens
	}
	return profile
}
