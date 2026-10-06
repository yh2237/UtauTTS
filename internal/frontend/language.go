package frontend

import "strings"

// NormalizeLanguageは言語コードをja/en/zhへ揃える。空は日本語。
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

// JapanesePlanは言語または音素化器が日本語かを返す。
func JapanesePlan(language, phonemizer string) bool {
	if NormalizeLanguage(language) == LanguageJapanese {
		return true
	}
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(phonemizer)), LanguageJapanese)
}
