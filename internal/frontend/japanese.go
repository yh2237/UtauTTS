package frontend

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/ikawaha/kagome-dict/ipa"
	"github.com/ikawaha/kagome/v2/tokenizer"

	"utautts/internal/frontend/ipapron"
)

var (
	japaneseOnce           sync.Once
	japaneseTokenizer      *tokenizer.Tokenizer
	japanesePronunciations *ipapron.Table
	japaneseError          error
)

// 既知語の発音は小さな表から引き、IPA辞書の全素性を読み込まない。
// 表が辞書と一致しない場合だけ全素性を持つ辞書へ戻す。
func loadJapaneseTokenizer() {
	dictionary := ipa.DictShrink()
	table, err := ipapron.Load(dictionary)
	if err != nil {
		dictionary, table = ipa.Dict(), nil
	}
	japanesePronunciations = table
	japaneseTokenizer, japaneseError = tokenizer.New(dictionary, tokenizer.OmitBosEos())
}

func tokenPronunciation(token tokenizer.Token, table *ipapron.Table) (string, bool) {
	if token.Class == tokenizer.KNOWN && table != nil {
		return table.Pronunciation(token.ID)
	}
	return token.Pronunciation()
}

func ToKana(text string) (string, error) {
	return ToKanaWithDictionary(text, nil)
}

func ToKanaWithDictionary(text string, dictionary map[string]string) (string, error) {
	replacements := dictionaryReplacements(dictionary)
	if len(replacements) == 0 {
		return toKana(text)
	}

	var result strings.Builder
	var ordinary strings.Builder
	flushOrdinary := func() error {
		if ordinary.Len() == 0 {
			return nil
		}
		segment := ordinary.String()
		ordinary.Reset()
		if strings.TrimSpace(segment) == "" {
			return nil
		}
		reading, err := toKana(segment)
		if err != nil {
			return err
		}
		result.WriteString(reading)
		return nil
	}
	for index := 0; index < len(text); {
		matched := false
		for _, item := range replacements {
			if !strings.HasPrefix(text[index:], item.surface) {
				continue
			}
			if err := flushOrdinary(); err != nil {
				return "", err
			}
			result.WriteString(normalizeDictionaryReading(item.reading))
			index += len(item.surface)
			matched = true
			break
		}
		if matched {
			continue
		}
		_, size := utf8.DecodeRuneInString(text[index:])
		if size == 0 {
			size = 1
		}
		ordinary.WriteString(text[index : index+size])
		index += size
	}
	if err := flushOrdinary(); err != nil {
		return "", err
	}
	return result.String(), nil
}

type replacement struct {
	surface string
	reading string
}

func dictionaryReplacements(dictionary map[string]string) []replacement {
	replacements := make([]replacement, 0, len(dictionary))
	for surface, reading := range dictionary {
		if strings.TrimSpace(surface) == "" || strings.TrimSpace(reading) == "" {
			continue
		}
		replacements = append(replacements, replacement{surface: surface, reading: reading})
	}
	if len(replacements) == 0 {
		return nil
	}
	sort.Slice(replacements, func(i, j int) bool {
		if len(replacements[i].surface) != len(replacements[j].surface) {
			return len(replacements[i].surface) > len(replacements[j].surface)
		}
		return replacements[i].surface < replacements[j].surface
	})
	return replacements
}

func ApplyDictionary(text string, dictionary map[string]string) string {
	if text == "" || len(dictionary) == 0 {
		return text
	}
	replacements := dictionaryReplacements(dictionary)
	if len(replacements) == 0 {
		return text
	}

	var result strings.Builder
	result.Grow(len(text))
	for index := 0; index < len(text); {
		matched := false
		for _, item := range replacements {
			if !strings.HasPrefix(text[index:], item.surface) {
				continue
			}
			result.WriteString(item.reading)
			index += len(item.surface)
			matched = true
			break
		}
		if matched {
			continue
		}
		_, size := utf8.DecodeRuneInString(text[index:])
		if size == 0 {
			size = 1
		}
		result.WriteString(text[index : index+size])
		index += size
	}
	return result.String()
}

// ApplyDictionaryForAnalysisは、辞書の読みを助詞として再解釈されにくいカタカナで本文へ埋め込む。
func ApplyDictionaryForAnalysis(text string, dictionary map[string]string) string {
	replacements := dictionaryReplacements(dictionary)
	if text == "" || len(replacements) == 0 {
		return text
	}
	converted := make(map[string]string, len(replacements))
	for _, item := range replacements {
		converted[item.surface] = normalizeDictionaryReading(item.reading)
	}
	return ApplyDictionary(text, converted)
}

func normalizeDictionaryReading(reading string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'ぁ' && r <= 'ゖ' {
			return r + 0x60
		}
		return r
	}, strings.TrimSpace(reading))
}

func toKana(text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("empty text")
	}
	japaneseOnce.Do(loadJapaneseTokenizer)
	if japaneseError != nil {
		return "", japaneseError
	}
	return readTokens(japaneseTokenizer.Tokenize(text), japanesePronunciations)
}

func readTokens(tokens []tokenizer.Token, table *ipapron.Table) (string, error) {
	var reading strings.Builder
	for _, token := range tokens {
		if pronunciation, ok := tokenPronunciation(token, table); ok && pronunciation != "" && pronunciation != "*" {
			reading.WriteString(pronunciation)
			continue
		}
		if safeSurface(token.Surface) {
			reading.WriteString(token.Surface)
			continue
		}
		if surfaceMayHavePronunciation(token.Surface) {
			return "", fmt.Errorf("no pronunciation for token %q", token.Surface)
		}
		// 絵文字など、読みを持たない部分は発話に含めない。
		continue
	}
	return reading.String(), nil
}

func surfaceMayHavePronunciation(surface string) bool {
	for _, r := range surface {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return true
		}
	}
	return false
}

func safeSurface(surface string) bool {
	for _, r := range surface {
		if unicode.IsSpace(r) || isKana(r) || strings.ContainsRune("、。，．,.!?！？・", r) {
			continue
		}
		return false
	}
	return true
}
