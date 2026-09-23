package frontend

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

type Mora struct {
	Language   string  `json:",omitempty"`
	WordIndex  int     `json:",omitempty"`
	WordEnd    bool    `json:",omitempty"`
	SourceText string  `json:",omitempty"`
	Phones     []Phone `json:",omitempty"`
	Text       string
	Consonant  string
	Vowel      string
	Pause      bool
	// PauseKindはポーズの種類（comma, period, question, ellipsis, space, bracket, other）。
	PauseKind     string `json:",omitempty"`
	Stress        int
	StressKnown   bool `json:",omitempty"`
	Tone          int
	DurationScale float64
	Aliases       *AliasHints
}

// ポーズの種類。句読点の種類ごとに休止長を変えるために保持する。
const (
	PauseKindComma    = "comma"
	PauseKindPeriod   = "period"
	PauseKindQuestion = "question"
	PauseKindEllipsis = "ellipsis"
	PauseKindSpace    = "space"
	PauseKindBracket  = "bracket"
	PauseKindOther    = "other"
)

// pauseKindRanksは連続する句読点を1つにまとめるときの優先度。値が大きいほど強い。
var pauseKindRanks = map[string]int{
	PauseKindOther:    1,
	PauseKindBracket:  1,
	PauseKindSpace:    2,
	PauseKindComma:    3,
	PauseKindPeriod:   4,
	PauseKindEllipsis: 5,
	PauseKindQuestion: 6,
}

type AliasHints struct {
	Main       []string
	MainKinds  []string
	Transition []string
	Endings    [][]string
	// EndingPhonesは発音に必要な語末音素を示す。
	EndingPhones [][]string `json:",omitempty"`
	// MainMissingは短縮CVで欠ける語頭子音を示す。
	MainMissing map[string][]string `json:",omitempty"`
}

// Phoneはalias表記に依存しない音素を示す。
type Phone struct {
	Symbol string
	Role   string
}

func ParseKana(reading string) ([]Mora, error) {
	reading = norm.NFC.String(reading)
	var result []Mora
	for _, r := range reading {
		if unicode.IsSpace(r) || strings.ContainsRune("、。，．,.!?！？・…‥〜～（）「」『』", r) {
			kind := pauseKindOf(r)
			if len(result) > 0 && !result[len(result)-1].Pause {
				result = append(result, Mora{Pause: true, PauseKind: kind})
			} else if len(result) > 0 {
				// 連続する句読点は1つのポーズにまとめ、強い方の種類を採用する。
				result[len(result)-1].PauseKind = strongerPauseKind(result[len(result)-1].PauseKind, kind)
			}
			continue
		}
		if !isKana(r) {
			continue
		}

		hiragana := toHiragana(r)
		if isCombiningSmallKana(hiragana) && len(result) > 0 && !result[len(result)-1].Pause {
			result[len(result)-1].Text += string(hiragana)
			result[len(result)-1].Vowel = vowelOf(hiragana, result[len(result)-1].Vowel)
			continue
		}
		if hiragana == 'ー' {
			if len(result) == 0 || result[len(result)-1].Pause {
				return nil, fmt.Errorf("long vowel mark has no preceding mora")
			}
			result = append(result, Mora{Text: "ー", Vowel: result[len(result)-1].Vowel})
			continue
		}
		result = append(result, Mora{Text: string(hiragana), Vowel: vowelOf(hiragana, "")})
	}
	for index := range result {
		if !result[index].Pause {
			result[index].Consonant = consonantOf(result[index].Text)
		}
	}
	return result, nil
}

func isKana(r rune) bool {
	return unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || r == 'ー'
}

// pauseKindOfは句読点・空白・括弧をポーズ種別へ分類する。
func pauseKindOf(r rune) string {
	switch {
	case strings.ContainsRune("、，,", r):
		return PauseKindComma
	case strings.ContainsRune("。．.", r):
		return PauseKindPeriod
	case strings.ContainsRune("？?！!", r):
		return PauseKindQuestion
	case strings.ContainsRune("…‥〜～", r):
		return PauseKindEllipsis
	case unicode.IsSpace(r):
		return PauseKindSpace
	case strings.ContainsRune("（）「」『』", r):
		return PauseKindBracket
	default:
		return PauseKindOther
	}
}

// strongerPauseKindは2つのポーズ種別のうち優先度が高い方を返す。
func strongerPauseKind(current, candidate string) string {
	if pauseKindRanks[candidate] > pauseKindRanks[current] {
		return candidate
	}
	return current
}

func toHiragana(r rune) rune {
	if r >= 'ァ' && r <= 'ヶ' {
		return r - 0x60
	}
	return r
}

func isCombiningSmallKana(r rune) bool {
	return strings.ContainsRune("ぁぃぅぇぉゃゅょゎ", r)
}

func vowelOf(r rune, fallback string) string {
	switch {
	case strings.ContainsRune("あかがさざただなはばぱまやらわぁゃゎ", r):
		return "a"
	case strings.ContainsRune("いきぎしじちぢにひびぴみりゐぃ", r):
		return "i"
	case strings.ContainsRune("うくぐすずつづぬふぶぷむゆるゔぅゅ", r):
		return "u"
	case strings.ContainsRune("えけげせぜてでねへべぺめれゑぇ", r):
		return "e"
	case strings.ContainsRune("おこごそぞとどのほぼぽもよろをぉょ", r):
		return "o"
	case r == 'ん':
		return "n"
	case r == 'っ':
		return "cl"
	default:
		return fallback
	}
}

func consonantOf(value string) string {
	if value == "" || value == "ー" {
		return ""
	}
	value = strings.Map(func(r rune) rune {
		if r >= 'ァ' && r <= 'ヶ' {
			return r - 0x60
		}
		return r
	}, value)
	if consonant, ok := japaneseMoraConsonants[value]; ok {
		return consonant
	}
	return ""
}

func ConsonantOf(value string) string {
	return consonantOf(value)
}

var japaneseMoraConsonants = map[string]string{
	"か": "k", "き": "k", "く": "k", "け": "k", "こ": "k",
	"が": "g", "ぎ": "g", "ぐ": "g", "げ": "g", "ご": "g",
	"きゃ": "ky", "きゅ": "ky", "きょ": "ky",
	"ぎゃ": "gy", "ぎゅ": "gy", "ぎょ": "gy",
	"さ": "s", "す": "s", "せ": "s", "そ": "s",
	"し": "sh", "しゃ": "sh", "しゅ": "sh", "しょ": "sh",
	"ざ": "z", "ず": "z", "ぜ": "z", "ぞ": "z",
	"じ": "j", "じゃ": "j", "じゅ": "j", "じょ": "j",
	"た": "t", "て": "t", "と": "t",
	"ち": "ch", "ちゃ": "ch", "ちゅ": "ch", "ちょ": "ch",
	"つ": "ts", "つぁ": "ts", "つぃ": "ts", "つぇ": "ts", "つぉ": "ts",
	"だ": "d", "で": "d", "ど": "d",
	"ぢ": "j", "ぢゃ": "j", "ぢゅ": "j", "ぢょ": "j",
	"な": "n", "に": "n", "ぬ": "n", "ね": "n", "の": "n",
	"にゃ": "ny", "にゅ": "ny", "にょ": "ny",
	"は": "h", "ひ": "h", "ふ": "f", "へ": "h", "ほ": "h",
	"ひゃ": "hy", "ひゅ": "hy", "ひょ": "hy",
	"ば": "b", "び": "b", "ぶ": "b", "べ": "b", "ぼ": "b",
	"びゃ": "by", "びゅ": "by", "びょ": "by",
	"ぱ": "p", "ぴ": "p", "ぷ": "p", "ぺ": "p", "ぽ": "p",
	"ぴゃ": "py", "ぴゅ": "py", "ぴょ": "py",
	"ま": "m", "み": "m", "む": "m", "め": "m", "も": "m",
	"みゃ": "my", "みゅ": "my", "みょ": "my",
	"や": "y", "ゆ": "y", "よ": "y",
	"ら": "r", "り": "r", "る": "r", "れ": "r", "ろ": "r",
	"りゃ": "ry", "りゅ": "ry", "りょ": "ry",
	"わ": "w", "を": "w",
	"ゔ": "v", "ゔぁ": "v", "ゔぃ": "v", "ゔぇ": "v", "ゔぉ": "v",
	"ゔゃ": "vy", "ゔゅ": "vy", "ゔょ": "vy",
	"ん": "n", "っ": "cl",
}
