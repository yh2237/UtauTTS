package g2p

import (
	"strconv"
	"strings"
)

var enSmall = strings.Fields("zero one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen sixteen seventeen eighteen nineteen")
var enTens = strings.Fields("zero ten twenty thirty forty fifty sixty seventy eighty ninety")

// EnglishCardinalは0以上の整数を英語の語にする。
func EnglishCardinal(n uint64) string {
	if n < 20 {
		return enSmall[n]
	}
	if n < 100 {
		result := enTens[n/10]
		if n%10 != 0 {
			result += " " + enSmall[n%10]
		}
		return result
	}
	for _, group := range []struct {
		value uint64
		name  string
	}{{1000000000000, "trillion"}, {1000000000, "billion"}, {1000000, "million"}, {1000, "thousand"}, {100, "hundred"}} {
		if n >= group.value {
			result := EnglishCardinal(n/group.value) + " " + group.name
			if n%group.value != 0 {
				result += " " + EnglishCardinal(n%group.value)
			}
			return result
		}
	}
	return ""
}

// EnglishNumberは数字の並びを英語の語にする。小数点以下は1桁ずつ読む。
func EnglishNumber(s string) string {
	integer, fraction, decimal := strings.Cut(s, ".")
	n, err := strconv.ParseUint(integer, 10, 64)
	result := ""
	if err != nil || len(integer) > 15 || (len(integer) > 1 && integer[0] == '0') {
		for _, digit := range integer {
			result += " " + enSmall[digit-'0']
		}
	} else {
		result = EnglishCardinal(n)
	}
	if decimal {
		result += " point"
		for _, digit := range fraction {
			result += " " + enSmall[digit-'0']
		}
	}
	return strings.TrimSpace(result)
}
