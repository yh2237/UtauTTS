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
	if strings.Contains(configured, "xsampa") {
		return LanguageEnglish, "en-delta"
	}
	// OpenUtauの英語の発音変換は、音源の直下にある自分の設定ファイルを読む。
	for _, config := range []struct{ file, phonemizer string }{
		{"envccv.yaml", "en-vccv"}, {"en-xsampa.yaml", "en-delta"}, {"en-cPv.yaml", "en-cv"},
	} {
		if b.Root != "" && findRootFile(b.Root, config.file) != "" {
			return LanguageEnglish, config.phonemizer
		}
	}
	// 音高ごとの収録は、別名に接頭辞・接尾辞が付く。
	affixes := []Affix{{}}
	for _, subbank := range b.Subbanks {
		affixes = append(affixes, Affix{Prefix: subbank.Prefix, Suffix: subbank.Suffix})
	}
	for _, affix := range b.PrefixMap {
		affixes = append(affixes, affix)
	}
	// prefix.mapのない音源は、別名の末尾に音高（B3、A#3など）を付ける。
	bases := map[string]bool{}
	for alias := range b.Entries {
		bases[alias] = true
		if base, _, ok := splitToneSuffix(alias); ok {
			bases[base] = true
		}
	}
	has := func(aliases ...string) bool {
		for _, alias := range aliases {
			for _, affix := range affixes {
				key := affix.Prefix + alias + affix.Suffix
				if len(b.Entries[key]) > 0 || bases[key] {
					return true
				}
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
	// ARPAsingは「- hh」「hh ah」のような子音文脈の別名を持つ。単音だけのC+Vより先に見る。
	if has("- hh", "hh ah", "ah l") || len(b.ARPAsing) > 0 {
		return LanguageEnglish, "en-arpasing"
	}
	// OpenUtauのC+V（en-cPv.yaml）は、ARPAbetの子音と母音を区切りなしの単音で録音する。
	if has("aa", "ah", "ao", "eh", "ih", "iy") && has("hh", "jh", "dh", "ng") {
		return LanguageEnglish, "en-cv"
	}
	if has("- ni", "ni") && has("hao", "- hao") {
		return LanguageChinese, "zh-cvvc"
	}
	return LanguageJapanese, "ja-kana"
}

// splitToneSuffixは別名の末尾の音高（B3、A#3など）を分ける。
func splitToneSuffix(alias string) (string, string, bool) {
	for size := 3; size >= 2; size-- {
		if len(alias) <= size {
			continue
		}
		suffix := alias[len(alias)-size:]
		if _, ok := toneNumber(suffix); ok {
			return alias[:len(alias)-size], suffix, true
		}
	}
	return alias, "", false
}
