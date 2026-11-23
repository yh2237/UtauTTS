package zh

import (
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/text/unicode/norm"
)

var (
	groupedNumberPattern = regexp.MustCompile(`\b[0-9]{1,3}(?:,[0-9]{3})+\b`)
	percentPattern       = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s*%`)
	numberPattern        = regexp.MustCompile(`[0-9]+(?:\.[0-9]+)?`)
)

var chineseDigits = []rune("零一二三四五六七八九")

func chineseCardinal(n uint64) string {
	if n < 10 {
		return string(chineseDigits[n])
	}
	for _, group := range []struct {
		value uint64
		name  string
	}{{100000000, "亿"}, {10000, "万"}, {1000, "千"}, {100, "百"}, {10, "十"}} {
		if n >= group.value {
			head := chineseCardinal(n / group.value)
			if group.value == 10 && n < 20 {
				head = ""
			}
			result := head + group.name
			rest := n % group.value
			if rest != 0 {
				if rest < group.value/10 {
					result += "零"
				}
				tail := chineseCardinal(rest)
				if rest >= 10 && rest < 20 && group.value > 10 {
					tail = "一" + tail
				}
				result += tail
			}
			return result
		}
	}
	return ""
}

// Normalizeは中国語の数字・年号・割合を読み用の漢字表現へ広げる。
func Normalize(text string) string {
	text = norm.NFKC.String(text)
	text = groupedNumberPattern.ReplaceAllStringFunc(text, func(s string) string { return strings.ReplaceAll(s, ",", "") })
	text = percentPattern.ReplaceAllString(text, "百分之$1")
	return numberPattern.ReplaceAllStringFunc(text, func(s string) string {
		integer, fraction, decimal := strings.Cut(s, ".")
		n, err := strconv.ParseUint(integer, 10, 64)
		// 年や長い識別子は1桁ずつ読む。
		digitwise := err != nil || len(integer) > 8 || (len(integer) > 1 && integer[0] == '0') || strings.Contains(text, s+"年")
		value := ""
		if digitwise {
			for _, r := range integer {
				value += string(chineseDigits[r-'0'])
			}
		} else {
			value = chineseCardinal(n)
		}
		if decimal {
			value += "点"
			for _, r := range fraction {
				value += string(chineseDigits[r-'0'])
			}
		}
		return value
	})
}
