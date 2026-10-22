package frontend

import "strings"

// 日本語のKokoroの記号。misaki（Apache-2.0）がcutlet（MIT、Paul O'Leary McCann）から移した表を、そのまま移す。
// かな1文字か、小書きのかなとの2文字を記号にする。っ、ん、ーは前後で決める。
var kokoroKana = map[string]string{
	"ぁ": "a", "あ": "a", "ぃ": "i", "い": "i", "ぅ": "ɯ", "う": "ɯ",
	"ぇ": "e", "え": "e", "ぉ": "o", "お": "o", "か": "ka", "が": "ɡa",
	"き": "kʲi", "ぎ": "ɡʲi", "く": "kɯ", "ぐ": "ɡɯ", "け": "ke", "げ": "ɡe",
	"こ": "ko", "ご": "ɡo", "さ": "sa", "ざ": "ʣa", "し": "ɕi", "じ": "ʥi",
	"す": "sɨ", "ず": "zɨ", "せ": "se", "ぜ": "ʣe", "そ": "so", "ぞ": "ʣo",
	"た": "ta", "だ": "da", "ち": "ʨi", "ぢ": "ʥi", "つ": "ʦɨ", "づ": "zɨ",
	"て": "te", "で": "de", "と": "to", "ど": "do", "な": "na", "に": "ɲi",
	"ぬ": "nɯ", "ね": "ne", "の": "no", "は": "ha", "ば": "ba", "ぱ": "pa",
	"ひ": "çi", "び": "bʲi", "ぴ": "pʲi", "ふ": "ɸɯ", "ぶ": "bɯ", "ぷ": "pɯ",
	"へ": "he", "べ": "be", "ぺ": "pe", "ほ": "ho", "ぼ": "bo", "ぽ": "po",
	"ま": "ma", "み": "mʲi", "む": "mɯ", "め": "me", "も": "mo", "ゃ": "ja",
	"や": "ja", "ゅ": "jɯ", "ゆ": "jɯ", "ょ": "jo", "よ": "jo", "ら": "ɾa",
	"り": "ɾʲi", "る": "ɾɯ", "れ": "ɾe", "ろ": "ɾo", "ゎ": "βa", "わ": "βa",
	"ゐ": "i", "ゑ": "e", "を": "o", "ゔ": "vɯ", "ゕ": "ka", "ゖ": "ke",
	"゙": "", "゚": "", "いぇ": "je", "うぃ": "βi", "うぇ": "βe", "うぉ": "βo",
	"きぇ": "kʲe", "きゃ": "kʲa", "きゅ": "kʲɨ", "きょ": "kʲo", "ぎゃ": "ɡʲa", "ぎゅ": "ɡʲɨ",
	"ぎょ": "ɡʲo", "くぁ": "kᵝa", "くぃ": "kᵝi", "くぇ": "kᵝe", "くぉ": "kᵝo", "ぐぁ": "ɡᵝa",
	"ぐぃ": "ɡᵝi", "ぐぇ": "ɡᵝe", "ぐぉ": "ɡᵝo", "しぇ": "ɕe", "しゃ": "ɕa", "しゅ": "ɕɨ",
	"しょ": "ɕo", "じぇ": "ʥe", "じゃ": "ʥa", "じゅ": "ʥɨ", "じょ": "ʥo", "ちぇ": "ʨe",
	"ちゃ": "ʨa", "ちゅ": "ʨɨ", "ちょ": "ʨo", "ぢゃ": "ʥa", "ぢゅ": "ʥɨ", "ぢょ": "ʥo",
	"つぁ": "ʦa", "つぃ": "ʦʲi", "つぇ": "ʦe", "つぉ": "ʦo", "てぃ": "tʲi", "てゅ": "tʲɨ",
	"でぃ": "dʲi", "でゅ": "dʲɨ", "とぅ": "tɯ", "どぅ": "dɯ", "にぇ": "ɲe", "にゃ": "ɲa",
	"にゅ": "ɲɨ", "にょ": "ɲo", "ひぇ": "çe", "ひゃ": "ça", "ひゅ": "çɨ", "ひょ": "ço",
	"びゃ": "bʲa", "びゅ": "bʲɨ", "びょ": "bʲo", "ぴゃ": "pʲa", "ぴゅ": "pʲɨ", "ぴょ": "pʲo",
	"ふぁ": "ɸa", "ふぃ": "ɸʲi", "ふぇ": "ɸe", "ふぉ": "ɸo", "ふゅ": "ɸʲɨ", "ふょ": "ɸʲo",
	"みゃ": "mʲa", "みゅ": "mʲɨ", "みょ": "mʲo", "りゃ": "ɾʲa", "りゅ": "ɾʲɨ", "りょ": "ɾʲo",
	"ゔぁ": "va", "ゔぃ": "vʲi", "ゔぇ": "ve", "ゔぉ": "vo", "ゔゅ": "bʲɨ", "ゔょ": "bʲo",
}

// KokoroJapaneseSymbolsは、日本語のモーラ（かな）ごとのKokoroの記号を返す。休止のモーラは句読点の記号にする。
// wordStartは各モーラが語の頭か（nilなら語の区切りなし）。spaceBeforeは、そのモーラの前に空白を置くか。
func KokoroJapaneseSymbols(morae []Mora, wordStart []bool) (symbols []string, spaceBefore []bool) {
	symbols = make([]string, len(morae))
	spaceBefore = make([]bool, len(morae))
	startsWord := func(i int) bool { return wordStart != nil && i < len(wordStart) && wordStart[i] }
	for i, mora := range morae {
		if mora.Pause {
			symbols[i] = kokoroPause(mora)
			continue
		}
		kana := katakanaToHiragana(mora.Text)
		switch kana {
		case "ー":
			symbols[i] = "ː"
		case "っ":
			symbols[i] = "ʔ"
		case "ん":
			next := ""
			if i+1 < len(morae) && !morae[i+1].Pause && !startsWord(i+1) {
				next = kokoroKanaSymbol(katakanaToHiragana(morae[i+1].Text))
			}
			symbols[i] = kokoroMoraicNasal(next)
		case "い":
			// 同じ語の中でi、eに続くいは長音（いい → iː、映画 → eːɡa）。misakiが使うUniDicの発音形に合わせる。
			if i > 0 && !morae[i-1].Pause && !startsWord(i) && (strings.HasSuffix(symbols[i-1], "i") || strings.HasSuffix(symbols[i-1], "e")) {
				symbols[i] = "ː"
			} else {
				symbols[i] = kokoroKanaSymbol(kana)
			}
		default:
			symbols[i] = kokoroKanaSymbol(kana)
		}
	}
	for i := 1; i < len(morae); i++ {
		switch {
		case morae[i].Pause:
		case morae[i-1].Pause:
			spaceBefore[i] = true
		case startsWord(i) && symbols[i] != "ʔ" && symbols[i-1] != "ʔ" && symbols[i] != "ː":
			spaceBefore[i] = true
		}
	}
	return symbols, spaceBefore
}

func kokoroKanaSymbol(kana string) string {
	if symbol, ok := kokoroKana[kana]; ok {
		return symbol
	}
	// 表にない小書きのかなとの組み合わせは、前のかなの子音に小書きのかなの記号を続ける（cutletと同じ）。
	runes := []rune(kana)
	if len(runes) == 2 {
		head, tail := kokoroKana[string(runes[0])], kokoroKana[string(runes[1])]
		if head != "" && tail != "" {
			return head[:len(head)-len(string([]rune(head)[len([]rune(head))-1]))] + tail
		}
	}
	return ""
}

// kokoroMoraicNasalは、ん の記号を次のかなの子音で決める（cutletと同じ）。
func kokoroMoraicNasal(next string) string {
	switch {
	case next == "":
		return "ɴ"
	case strings.ContainsAny(next[:1], "mpb"):
		return "m"
	case strings.HasPrefix(next, "k") || strings.HasPrefix(next, "ɡ"):
		return "ŋ"
	case strings.HasPrefix(next, "ɲ") || strings.HasPrefix(next, "ʨ") || strings.HasPrefix(next, "ʥ"):
		return "ɲ"
	case strings.ContainsAny(next[:1], "ntdz") || strings.HasPrefix(next, "ɾ"):
		return "n"
	}
	return "ɴ"
}

func kokoroPause(mora Mora) string {
	switch mora.PauseKind {
	case PauseKindQuestion:
		return "?"
	case PauseKindPeriod:
		return "."
	}
	if strings.ContainsAny(mora.Text, "！!") {
		return "!"
	}
	return ","
}

func katakanaToHiragana(text string) string {
	runes := []rune(text)
	for i, r := range runes {
		if r >= 'ァ' && r <= 'ヶ' {
			runes[i] = r - 0x60
		}
	}
	return string(runes)
}
