package en

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

func (analyzer) Language() string          { return linguistics.LanguageEnglish }
func (analyzer) DefaultPhonemizer() string { return linguistics.PhonemizerEnglish }
func (analyzer) Phonemizers() []string {
	return []string{
		linguistics.PhonemizerEnglish,
		linguistics.PhonemizerEnglishDelta,
		linguistics.PhonemizerEnglishVCCV,
		linguistics.PhonemizerEnglishCV,
	}
}
func (analyzer) Normalize(text string) string { return Normalize(text) }

func (analyzer) Analyze(_ context.Context, input g2p.Input) (linguistics.Utterance, error) {
	phonemizer := strings.ToLower(strings.TrimSpace(input.Phonemizer))
	if phonemizer == "" {
		phonemizer = linguistics.PhonemizerEnglish
	}
	return Parse(phonemizer, input.Text, input.Reading, input.Dictionary)
}

// Parseは英語の文またはARPAbet読みを指定された発音形式の粒度（音素単位／音節単位）で解析する。
func Parse(phonemizer, text, reading string, dictionary map[string]string) (linguistics.Utterance, error) {
	phonemizer = strings.ToLower(strings.TrimSpace(phonemizer))
	if phonemizer == "" {
		phonemizer = linguistics.PhonemizerEnglish
	}
	wordsText := Words(text)
	var pronunciation, automatic string
	var parsedWords [][]string
	if strings.TrimSpace(reading) != "" {
		pronunciation = strings.TrimSpace(reading)
		var err error
		parsedWords, err = parseExplicitReading(pronunciation)
		if err != nil {
			return linguistics.Utterance{}, err
		}
		if auto, _, err := wordsPronunciation(wordsText, dictionary); err == nil {
			automatic = auto
		}
	} else {
		var err error
		pronunciation, parsedWords, err = wordsPronunciation(wordsText, dictionary)
		if err != nil {
			return linguistics.Utterance{}, err
		}
		automatic = pronunciation
	}

	var morae []linguistics.Mora
	var err error
	switch phonemizer {
	case linguistics.PhonemizerEnglish:
		morae, err = buildPhoneMorae(parsedWords, false)
	case linguistics.PhonemizerEnglishCV:
		morae, err = buildPhoneMorae(parsedWords, true)
	case linguistics.PhonemizerEnglishDelta, linguistics.PhonemizerEnglishVCCV:
		morae, err = buildSyllableMorae(parsedWords)
	default:
		return linguistics.Utterance{}, fmt.Errorf("unsupported phonemizer %q for language %q", phonemizer, linguistics.LanguageEnglish)
	}
	if err != nil {
		return linguistics.Utterance{}, err
	}
	return linguistics.Utterance{
		Language:   linguistics.LanguageEnglish,
		Phonemizer: phonemizer,
		Text:       text,
		Reading:    pronunciation,
		Automatic:  automatic,
		Words:      wordsText,
		Morae:      morae,
	}, nil
}

func buildPhoneMorae(words [][]string, allowCVConsonants bool) ([]linguistics.Mora, error) {
	var morae []linguistics.Mora
	for wordIndex, word := range words {
		if len(word) == 1 && word[0] == "SP" {
			if len(morae) > 0 && !morae[len(morae)-1].Pause {
				morae = append(morae, linguistics.Mora{Pause: true})
			}
			continue
		}
		for phoneIndex, raw := range word {
			symbol := NormalizeARPAbet(raw)
			if symbol == "" {
				continue
			}
			validConsonant := IsARPAbetConsonant(symbol)
			if allowCVConsonants {
				validConsonant = IsCVConsonant(symbol)
			}
			if !IsVowel(symbol) && !validConsonant {
				return nil, fmt.Errorf("unsupported ARPAbet phoneme %q", raw)
			}
			mora := linguistics.Mora{
				Language:    linguistics.LanguageEnglish,
				WordIndex:   wordIndex,
				WordEnd:     phoneIndex+1 == len(word),
				Text:        symbol,
				Stress:      ARPAbetStress(raw),
				StressKnown: ARPAbetHasStress(raw),
			}
			if IsVowel(symbol) {
				mora.Vowel = symbol
				mora.DurationScale = 1
				mora.Phones = []linguistics.Phone{{Symbol: symbol, Role: "nucleus"}}
			} else {
				mora.Consonant = symbol
				mora.DurationScale = linguistics.PhoneWeight(symbol, "onset")
				mora.Phones = []linguistics.Phone{{Symbol: symbol, Role: "onset"}}
			}
			morae = append(morae, mora)
		}
	}
	MarkPhoneCodas(morae)
	return morae, nil
}

func buildSyllableMorae(words [][]string) ([]linguistics.Mora, error) {
	var syllableWords [][]Syllable
	for _, word := range words {
		if len(word) == 1 && word[0] == "SP" {
			syllableWords = append(syllableWords, nil)
			continue
		}
		syllables, err := SyllabifyWord(word)
		if err != nil {
			return nil, err
		}
		if len(syllables) == 0 {
			if len(syllableWords) == 0 || len(syllableWords[len(syllableWords)-1]) == 0 {
				return nil, fmt.Errorf("ARPAbet reading contains no vowel")
			}
			lastWord := syllableWords[len(syllableWords)-1]
			lastWord[len(lastWord)-1].Coda = append(lastWord[len(lastWord)-1].Coda, NormalizePhones(word)...)
			continue
		}
		syllableWords = append(syllableWords, syllables)
	}
	if len(syllableWords) == 0 {
		return nil, fmt.Errorf("ARPAbet reading contains no vowel")
	}

	morae := make([]linguistics.Mora, 0)
	for wordIndex, syllables := range syllableWords {
		if len(syllables) == 0 {
			if len(morae) > 0 && !morae[len(morae)-1].Pause {
				morae = append(morae, linguistics.Mora{Pause: true})
			}
			continue
		}
		for syllableIndex, syllable := range syllables {
			phones := SyllablePhones(syllable)
			phoneSymbols := make([]string, len(phones))
			for i, phone := range phones {
				phoneSymbols[i] = phone.Symbol
			}
			morae = append(morae, linguistics.Mora{
				Language:    linguistics.LanguageEnglish,
				WordIndex:   wordIndex,
				WordEnd:     syllableIndex+1 == len(syllables),
				Phones:      phones,
				Text:        strings.Join(phoneSymbols, " "),
				Consonant:   strings.Join(syllable.Onset, " "),
				Vowel:       syllable.Vowel,
				Stress:      syllable.Stress,
				StressKnown: syllable.StressKnown,
			})
		}
	}
	return morae, nil
}
