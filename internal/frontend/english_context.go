package frontend

import "strings"

// 不定詞・命令・助動詞の後ではreadの原形を使う。
func englishReadIsBaseForm(words []string, index int) bool {
	if index == 0 || words[index-1] == "<pause>" {
		return true
	}
	switch strings.ToLower(words[index-1]) {
	case "please", "to", "can", "could", "will", "would", "shall", "should", "may", "might", "must", "do", "does", "did", "don't", "doesn't", "didn't":
		return true
	}
	return false
}
