package openjtalk

import (
	"strings"

	"utautts/internal/frontend"
)

var mfaConsonants = map[string]string{"k": "k", "g": "ɡ", "s": "s", "sh": "ɕ", "z": "z", "j": "dʑ", "t": "t", "ch": "tɕ", "ts": "ts", "d": "d", "n": "n", "h": "h", "f": "ɸ", "b": "b", "p": "p", "m": "m", "y": "j", "r": "ɾ", "w": "w", "v": "v", "dy": "dʲ", "ty": "tʲ", "ky": "c", "gy": "ɟ", "ny": "ɲ", "hy": "ç", "my": "mʲ", "ry": "ɾʲ", "by": "bʲ", "py": "pʲ"}
var mfaBeforeI = map[string]string{"k": "c", "g": "ɟ", "n": "ɲ", "h": "ç", "m": "mʲ", "r": "ɾʲ", "b": "bʲ", "p": "pʲ"}
var mfaForeignConsonants = map[string]string{"てぃ": "t", "でぃ": "d", "ふぁ": "f", "ふぃ": "f", "ふぇ": "f", "ふぉ": "f", "づ": "z", "を": ""}

// MoraPhones はMFA用の仮名をOpen JTalkの音素表記へ変換する。
// 独立した長音は前モーラを必要とするため呼び出し側で処理する。
func MoraPhones(text string) []string {
	morae, err := frontend.ParseKana(text)
	if err != nil || len(morae) == 0 {
		return nil
	}
	phones := []string{}
	previous := ""
	for _, m := range morae {
		if m.Pause {
			return nil
		}
		if m.Text == "ー" {
			if previous == "" {
				return nil
			}
			phones = append(phones, previous)
			continue
		}
		if m.Vowel == "cl" {
			phones = append(phones, "ʔ")
			continue
		}
		if m.Vowel == "n" {
			phones = append(phones, "ɴ")
			continue
		}
		vowel := m.Vowel
		if vowel == "u" {
			vowel = "ɯ"
		}
		if !strings.Contains("aiɯeo", vowel) || len([]rune(vowel)) != 1 {
			return nil
		}
		name := m.Consonant
		if replacement, ok := mfaForeignConsonants[m.Text]; ok {
			name = replacement
		}
		if name != "" {
			consonant, ok := mfaConsonants[name]
			if !ok {
				return nil
			}
			if m.Vowel == "i" {
				if special, ok := mfaBeforeI[name]; ok {
					consonant = special
				}
			}
			phones = append(phones, consonant)
		}
		phones = append(phones, vowel)
		previous = vowel
	}
	return phones
}
