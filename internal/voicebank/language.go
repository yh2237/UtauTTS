package voicebank

import "strings"

const (
	LanguageJapanese = "ja"
	LanguageEnglish  = "en"
	LanguageChinese  = "zh"
)

func (b *Bank) SuggestedLanguage() (string, string) {
	configured := strings.ToLower(b.DefaultPhonemizer)
	if strings.Contains(configured, "chinese") {
		return LanguageChinese, "zh-cvvc"
	}
	if strings.Contains(configured, "englishvccv") {
		return LanguageEnglish, "en-vccv"
	}
	if strings.Contains(configured, "arpasing") {
		return LanguageEnglish, "en-arpasing"
	}
	has := func(aliases ...string) bool {
		for _, alias := range aliases {
			if len(b.Entries[alias]) > 0 {
				return true
			}
		}
		return false
	}
	if has("-h@", "-hA", "-b&") {
		return LanguageEnglish, "en-vccv"
	}
	if has("- hV", "- h@", "V l", "@ l") && has("h{", "- h{") {
		return LanguageEnglish, "en-delta"
	}
	if has("- hh", "hh ah", "ah l") || len(b.ARPAsing) > 0 {
		return LanguageEnglish, "en-arpasing"
	}
	if has("- ni", "ni") && has("hao", "- hao") {
		return LanguageChinese, "zh-cvvc"
	}
	return LanguageJapanese, "ja-kana"
}
