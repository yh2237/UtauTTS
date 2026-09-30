package frontend

import "strings"

// EnglishOptionsは英語phonemizer共通の前処理オプション。
type EnglishOptions struct {
	// WeakFormsは非強調の機能語へ弱形を適用する(E1)。
	WeakForms bool
}

func DefaultEnglishOptions() EnglishOptions {
	return EnglishOptions{WeakForms: true}
}

// CMUdictとLadefoged「A Course in Phonetics」を参考にした機能語の弱形。
// AH0はenglishSyllableVowelsで曖昧母音AXへ展開する。
var englishWeakForms = map[string]string{
	"of":    "AH0 V",
	"and":   "AH0 N",
	"a":     "AH0",
	"an":    "AH0 N",
	"the":   "DH AH0",
	"to":    "T AH0",
	"for":   "F ER0",
	"from":  "F R AH0 M",
	"as":    "AH0 Z",
	"at":    "AH0 T",
	"but":   "B AH0 T",
	"can":   "K AH0 N",
	"could": "K UH0 D",
	"have":  "HH AH0 V",
	"has":   "HH AH0 Z",
	"had":   "HH AH0 D",
	"was":   "W AH0 Z",
	"were":  "W ER0",
	"do":    "D AH0",
	"does":  "D AH0 Z",
	"them":  "DH AH0 M",
	"than":  "DH AH0 N",
	"that":  "DH AH0 T",
	"his":   "HH IH0 Z",
	"her":   "HH ER0",
	"us":    "AH0 S",
	"some":  "S AH0 M",
	"your":  "Y ER0",
	"am":    "AH0 M",
	"are":   "ER0",
	"been":  "B IH0 N",
}

// 句中の機能語だけを弱形にする。明示の読みと辞書は呼び出し側で除外する。
func englishWeakForm(word string, words []string, index int, options EnglishOptions) (string, bool) {
	if !options.WeakForms {
		return "", false
	}
	if index == 0 || index+1 >= len(words) {
		return "", false
	}
	if words[index-1] == "<pause>" || words[index+1] == "<pause>" {
		return "", false
	}
	weak, ok := englishWeakForms[strings.ToLower(word)]
	return weak, ok
}
