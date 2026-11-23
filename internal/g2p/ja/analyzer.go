package ja

import (
	"context"
	"fmt"
	"strings"

	"github.com/yh2237/UtauTTS/internal/g2p"
	"github.com/yh2237/UtauTTS/internal/linguistics"
)

type analyzer struct{}

func init() {
	g2p.Register(analyzer{})
}

func (analyzer) Language() string          { return linguistics.LanguageJapanese }
func (analyzer) DefaultPhonemizer() string { return linguistics.PhonemizerJapanese }
func (analyzer) Phonemizers() []string     { return []string{linguistics.PhonemizerJapanese} }
func (analyzer) Normalize(text string) string {
	return text
}

func (analyzer) Analyze(ctx context.Context, input g2p.Input) (linguistics.Utterance, error) {
	phonemizer := strings.ToLower(strings.TrimSpace(input.Phonemizer))
	if phonemizer == "" {
		phonemizer = linguistics.PhonemizerJapanese
	}
	if phonemizer != linguistics.PhonemizerJapanese {
		return linguistics.Utterance{}, fmt.Errorf("unsupported phonemizer %q for language %q", phonemizer, linguistics.LanguageJapanese)
	}
	reading := input.Reading
	if strings.TrimSpace(reading) == "" {
		var err error
		reading, err = ToKanaWithDictionary(input.Text, input.Dictionary)
		if err != nil {
			if input.ReadingFallback == nil {
				return linguistics.Utterance{}, err
			}
			fallbackReading, fallbackErr := input.ReadingFallback(ctx, ApplyDictionaryForAnalysis(input.Text, input.Dictionary))
			if fallbackErr != nil {
				return linguistics.Utterance{}, fmt.Errorf("convert text to reading: %v; %w", err, fallbackErr)
			}
			reading = fallbackReading
		}
	}
	morae, err := linguistics.ParseKana(reading)
	if err != nil {
		return linguistics.Utterance{}, err
	}
	return linguistics.Utterance{
		Language:   linguistics.LanguageJapanese,
		Phonemizer: phonemizer,
		Text:       input.Text,
		Reading:    reading,
		Morae:      morae,
	}, nil
}
