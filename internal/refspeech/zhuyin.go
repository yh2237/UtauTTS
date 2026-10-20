package refspeech

import "strings"

// 中国語の音素の書き方。Kokoroの中国語モデル（hexgrad/Kokoro-82M-v1.1-zh）は、
// 声母と韻母を注音符号で、声調を1〜5の数字で書く。
// 表はmisakiの中国語G2P（zh_frontend.py の ZH_MAP）と同じ。
var zhuyinInitials = map[string]string{
	"b": "ㄅ", "p": "ㄆ", "m": "ㄇ", "f": "ㄈ", "d": "ㄉ", "t": "ㄊ", "n": "ㄋ", "l": "ㄌ",
	"g": "ㄍ", "k": "ㄎ", "h": "ㄏ", "j": "ㄐ", "q": "ㄑ", "x": "ㄒ", "zh": "ㄓ", "ch": "ㄔ",
	"sh": "ㄕ", "r": "ㄖ", "z": "ㄗ", "c": "ㄘ", "s": "ㄙ",
}

var zhuyinFinals = map[string]string{
	"a": "ㄚ", "o": "ㄛ", "e": "ㄜ", "ie": "ㄝ", "ai": "ㄞ", "ei": "ㄟ", "ao": "ㄠ", "ou": "ㄡ",
	"an": "ㄢ", "en": "ㄣ", "ang": "ㄤ", "eng": "ㄥ", "er": "ㄦ", "i": "ㄧ", "u": "ㄨ", "v": "ㄩ",
	"ii": "ㄭ", "iii": "十", "ve": "月", "ia": "压", "ian": "言", "iang": "阳", "iao": "要",
	"in": "阴", "ing": "应", "iong": "用", "iou": "又", "ong": "中", "ua": "穵", "uai": "外",
	"uan": "万", "uang": "王", "uei": "为", "uen": "文", "ueng": "瓮", "uo": "我", "van": "元",
	"vn": "云",
}

// pinyinSpellingsは、y・wで始まる綴りやüの綴りを、声母と韻母の組みに直す。
// 綴りをそのまま声母と韻母に分けると、注音にならないため。
var pinyinSpellings = map[string]string{
	"yi": "i", "ya": "ia", "ye": "ie", "yao": "iao", "you": "iou", "yan": "ian", "yin": "in",
	"yang": "iang", "ying": "ing", "yong": "iong", "yu": "v", "yue": "ve", "yuan": "van",
	"yun": "vn", "wu": "u", "wa": "ua", "wo": "uo", "wai": "uai", "wei": "uei", "wan": "uan",
	"wen": "uen", "wang": "uang", "weng": "ueng",
}

// pinyinInitialsは、長いものから順に並べる（zhをzより先に見る）。
var pinyinInitials = []string{
	"zh", "ch", "sh", "b", "p", "m", "f", "d", "t", "n", "l", "g", "k", "h", "j", "q", "x",
	"r", "z", "c", "s",
}

// pinyinToZhuyinは、声調付きのピンイン（例: ni3、zhong1、lv4）を注音＋声調にする。
func pinyinToZhuyin(syllable string) (string, bool) {
	syllable = strings.ToLower(strings.TrimSpace(syllable))
	if syllable == "" {
		return "", false
	}
	tone := ""
	if last := syllable[len(syllable)-1]; last >= '1' && last <= '5' {
		tone = string(last)
		syllable = syllable[:len(syllable)-1]
	}
	syllable = strings.ReplaceAll(syllable, "u:", "v")
	syllable = strings.ReplaceAll(syllable, "ü", "v")
	// j・q・xのあとのuはüと書く。
	for _, initial := range []string{"j", "q", "x"} {
		if strings.HasPrefix(syllable, initial) && strings.Contains(syllable, "u") {
			syllable = initial + strings.Replace(syllable[len(initial):], "u", "v", 1)
			break
		}
	}
	if expansion, ok := pinyinSpellings[syllable]; ok {
		syllable = expansion
	}
	initial, final := "", syllable
	for _, candidate := range pinyinInitials {
		if strings.HasPrefix(syllable, candidate) && len(syllable) > len(candidate) {
			initial, final = candidate, syllable[len(candidate):]
			break
		}
	}
	// zi・ci・siとzhi・chi・shi・riのiは、それぞれ違う韻母として書く。
	if final == "i" {
		switch initial {
		case "z", "c", "s":
			final = "ii"
		case "zh", "ch", "sh", "r":
			final = "iii"
		}
	}
	// 綴りの揺れを、表にある韻母に寄せる。
	switch final {
	case "iu":
		final = "iou"
	case "ui":
		final = "uei"
	case "un":
		final = "uen"
	case "ue":
		final = "ve"
	}
	initialSymbol, ok := zhuyinInitials[initial]
	if initial != "" && !ok {
		return "", false
	}
	finalSymbol, ok := zhuyinFinals[final]
	if !ok {
		return "", false
	}
	return initialSymbol + finalSymbol + tone, true
}

// zhuyinPhonemesは、ピンインの並びをKokoroの中国語の音素列にする。読めない音節は飛ばす。
func zhuyinPhonemes(pinyins []string) string {
	var builder strings.Builder
	for _, pinyin := range pinyins {
		if pinyin == "|" {
			builder.WriteString(",")
			continue
		}
		if symbol, ok := pinyinToZhuyin(pinyin); ok {
			builder.WriteString(symbol)
		}
	}
	return builder.String()
}
