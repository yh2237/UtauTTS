package frontend

import (
	"fmt"
	"strings"
)

const (
	LanguageJapanese   = "ja"
	LanguageEnglish    = "en"
	LanguageChinese    = "zh"
	LanguageSpanish    = "es"
	LanguagePortuguese = "pt"

	PhonemizerJapanese     = "ja-kana"
	PhonemizerEnglish      = "en-arpasing"
	PhonemizerEnglishDelta = "en-delta"
	PhonemizerEnglishVCCV  = "en-vccv"
	PhonemizerEnglishCV    = "en-cv"
	PhonemizerChinese      = "zh-cvvc"
	PhonemizerSpanish      = "es-syllable"
	PhonemizerPortuguese   = "pt-syllable"
)

func ResolveLanguage(language, phonemizer string) (string, string, error) {
	language = strings.ToLower(strings.TrimSpace(language))
	phonemizer = strings.ToLower(strings.TrimSpace(phonemizer))
	if language == "" {
		switch phonemizer {
		case PhonemizerEnglish, PhonemizerEnglishDelta, PhonemizerEnglishVCCV, PhonemizerEnglishCV:
			language = LanguageEnglish
		case PhonemizerChinese:
			language = LanguageChinese
		case PhonemizerSpanish:
			language = LanguageSpanish
		case PhonemizerPortuguese:
			language = LanguagePortuguese
		default:
			language = LanguageJapanese
		}
	}
	if phonemizer == "" {
		phonemizer = map[string]string{
			LanguageJapanese:   PhonemizerJapanese,
			LanguageEnglish:    PhonemizerEnglish,
			LanguageChinese:    PhonemizerChinese,
			LanguageSpanish:    PhonemizerSpanish,
			LanguagePortuguese: PhonemizerPortuguese,
		}[language]
	}
	valid := map[string]map[string]bool{
		LanguageJapanese:   {PhonemizerJapanese: true},
		LanguageEnglish:    {PhonemizerEnglish: true, PhonemizerEnglishDelta: true, PhonemizerEnglishVCCV: true, PhonemizerEnglishCV: true},
		LanguageChinese:    {PhonemizerChinese: true},
		LanguageSpanish:    {PhonemizerSpanish: true},
		LanguagePortuguese: {PhonemizerPortuguese: true},
	}[language]
	if valid == nil {
		return "", "", fmt.Errorf("unsupported language %q", language)
	}
	if !valid[phonemizer] {
		return "", "", fmt.Errorf("phonemizer %q does not support language %q", phonemizer, language)
	}
	return language, phonemizer, nil
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func repeatAliasKind(kind string, count int) []string {
	result := make([]string, count)
	for index := range result {
		result[index] = kind
	}
	return result
}
