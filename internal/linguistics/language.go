package linguistics

import "strings"

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

// NormalizeLanguageは言語コードをja/en/zh/es/ptへ揃える。空は日本語。
func NormalizeLanguage(language string) string {
	value := strings.ToLower(strings.TrimSpace(language))
	if value == "" {
		return LanguageJapanese
	}
	if index := strings.IndexAny(value, "-_"); index > 0 {
		value = value[:index]
	}
	if value == "cmn" {
		return LanguageChinese
	}
	return value
}

// JapanesePlanは言語または発音形式が日本語かを返す。
func JapanesePlan(language, phonemizer string) bool {
	if NormalizeLanguage(language) == LanguageJapanese {
		return true
	}
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(phonemizer)), LanguageJapanese)
}

// IsSyllableLanguageは音節を1モーラにして読む言語かを返す。
func IsSyllableLanguage(language string) bool {
	return language == LanguageSpanish || language == LanguagePortuguese
}
