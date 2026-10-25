package g2p

import "strings"

// foreignReadingsは、その言語の規則で読むと不自然になりやすい外来語・ブランド名の読み。
// 中国語はPinyin（声調付き）、スペイン語・ポルトガル語は音節を「.」で区切った読み。
var foreignReadings = map[Language]map[string]string{
	English: {
		"deepseek":   "D IY1 P S IY2 K",
		"anthropic":  "AE0 N TH R AA1 P IH0 K",
		"kubernetes": "K UW2 B ER0 N EH1 T IY0 Z",
		"pytorch":    "P AY1 T AO2 R CH",
		"postgresql": "P OW1 S T G R EH2 S K Y UW2 EH1 L",
		"postgres":   "P OW1 S T G R EH2 S",
		"postgre":    "P OW1 S T G R EH2",
		"mysql":      "M AY2 EH1 S K Y UW2 EH1 L",
		"sqlite":     "EH2 S K Y UW2 EH1 L AY2 T",
		"redis":      "R EH1 D IH0 S",
		"tensorflow": "T EH1 N S ER0 F L OW2",
		"midjourney": "M IH1 D JH ER0 N IY0",
		"uniqlo":     "Y UW1 N IY0 K L OW2",
		"rakuten":    "R AE1 K UW0 T EH2 N",
		"mercari":    "M ER0 K AA1 R IY0",
		"zundamon":   "Z AH0 N D AE1 M AH0 N",
	},
	Chinese: {
		"openai":    "ou1 pen1 ei1 ai4",
		"iphone":    "ai4 feng1",
		"google":    "gu3 ge1",
		"chatgpt":   "cha1 te4 ji4 pi4 ti4",
		"wifi":      "wai1 fei1",
		"youtube":   "you2 tu2 bu4",
		"whatsapp":  "wa1 ci1 a1 pu4",
		"facebook":  "fei1 si1 bu4",
		"tiktok":    "ti4 ke4 tuo4 ke4",
		"netflix":   "nai4 fei1 li4 ke4 si1",
		"amazon":    "ya4 ma3 xun4",
		"instagram": "yin1 si1 te4 ge1 la1 mu4",
		"spotify":   "si1 bo1 ti1 fei1",
		"telegram":  "te4 le4 ge1 la1 mu4",
		"linkedin":  "ling1 ke4 yin1",
		"wikipedia": "wei1 ji1 bai3 ke1",
	},
	Spanish: {
		"openai":    "ˌo.pen.ˈaj",
		"iphone":    "ˈaj.fon",
		"google":    "ˈɡu.ɣel",
		"chatgpt":   "ʧˈat.xe.pe.tˈe",
		"microsoft": "mˌi.kɾo.sˈoft",
		"apple":     "ˈap.ple",
		"youtube":   "ˈju.tub",
		"whatsapp":  "wˈat.sap",
		"facebook":  "fˈejz.buk",
		"tiktok":    "tˈik.tok",
		"netflix":   "nˈet.fliks",
		"amazon":    "ˈa.ma.son",
		"instagram": "ˌins.ta.ɡɾˈam",
		"spotify":   "es.po.ti.ˈfaj",
		"telegram":  "te.le.ɡɾˈam",
		"linkedin":  "lin.ke.ˈðin",
		"wikipedia": "wi.ki.ˈpe.ðja",
	},
	Portuguese: {
		"openai":    "ˌo.pen.ˈaj",
		"iphone":    "ˌaj.ˈfo.ne",
		"google":    "ˈɡu.ɡow",
		"chatgpt":   "ʃˈat.ʒe.pe.tˈe",
		"microsoft": "mˌi.kro.zˈoft",
		"apple":     "ˈɛ.pol",
		"youtube":   "ˈju.tu.bi",
		"whatsapp":  "wˈat.sap",
		"facebook":  "fˈejz.buk",
		"tiktok":    "tˈik.tok",
		"netflix":   "nˈet.fliks",
		"amazon":    "ˈa.ma.zon",
		"instagram": "ˌins.ta.ɡɾˈɐ̃m",
		"spotify":   "is.ˈpɔ.tʃi.faj",
		"telegram":  "te.le.ɡɾˈɐ̃m",
		"linkedin":  "lin.ke.ˈdʒĩ",
		"wikipedia": "wi.ki.ˈpɛ.dʒa",
	},
}

// ForeignReadingは、外来語・ブランド名の読みを返す。
func ForeignReading(language Language, word string) (string, bool) {
	entries := foreignReadings[language]
	if entries == nil {
		return "", false
	}
	reading, ok := entries[strings.ToLower(strings.TrimSpace(word))]
	return reading, ok
}
