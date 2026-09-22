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
	if strings.Contains(configured, "cpv") || strings.Contains(configured, "c+v") {
		return LanguageEnglish, "en-cv"
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
	// C+VはARPAbetの子音・母音を単音で録音し、-C/-V, C/V, C-/V- を持つ。
	// 子音文脈のCV/VCを含むARPAsingとは「V -」終端と単音母音の有無で区別する。
	if has("- aa", "- ah", "- ao") && has("aa -", "ah -", "ao -") && !has("hh ah", "ah l") {
		return LanguageEnglish, "en-cv"
	}
	if has("- hh", "hh ah", "ah l") || len(b.ARPAsing) > 0 {
		return LanguageEnglish, "en-arpasing"
	}
	if has("- ni", "ni") && has("hao", "- hao") {
		return LanguageChinese, "zh-cvvc"
	}
	return LanguageJapanese, "ja-kana"
}
