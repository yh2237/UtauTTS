package frontend

import (
	"golang.org/x/text/unicode/norm"
	"regexp"
	"strconv"
	"strings"
)

var numberPattern = regexp.MustCompile(`[0-9]+(?:\.[0-9]+)?`)
var timePattern = regexp.MustCompile(`\b([0-9]{1,2}):([0-9]{2})\b`)
var groupedNumberPattern = regexp.MustCompile(`\b[0-9]{1,3}(?:,[0-9]{3})+\b`)
var enSmall = strings.Fields("zero one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen sixteen seventeen eighteen nineteen")
var enTens = strings.Fields("zero ten twenty thirty forty fifty sixty seventy eighty ninety")

func englishCardinal(n uint64) string {
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
			result := englishCardinal(n/group.value) + " " + group.name
			if n%group.value != 0 {
				result += " " + englishCardinal(n%group.value)
			}
			return result
		}
	}
	return ""
}

func englishNumber(s string) string {
	integer, fraction, decimal := strings.Cut(s, ".")
	n, err := strconv.ParseUint(integer, 10, 64)
	result := ""
	if err != nil || len(integer) > 15 || (len(integer) > 1 && integer[0] == '0') {
		for _, digit := range integer {
			result += " " + enSmall[digit-'0']
		}
	} else {
		result = englishCardinal(n)
	}
	if decimal {
		result += " point"
		for _, digit := range fraction {
			result += " " + enSmall[digit-'0']
		}
	}
	return strings.TrimSpace(result)
}

func normalizeEnglishText(text string) string {
	text = norm.NFKC.String(text)
	text = groupedNumberPattern.ReplaceAllStringFunc(text, func(s string) string { return strings.ReplaceAll(s, ",", "") })
	text = strings.NewReplacer("’", "'", "％", "%").Replace(text)
	text = timePattern.ReplaceAllStringFunc(text, func(s string) string {
		parts := strings.Split(s, ":")
		hour, _ := strconv.Atoi(parts[0])
		minute, _ := strconv.Atoi(parts[1])
		if hour > 23 || minute > 59 {
			return s
		}
		value := englishNumber(parts[0])
		if minute == 0 {
			return value + " o'clock"
		}
		if minute < 10 {
			return value + " oh " + englishCardinal(uint64(minute))
		}
		return value + " " + englishCardinal(uint64(minute))
	})
	text = numberPattern.ReplaceAllStringFunc(text, func(s string) string { return " " + englishNumber(s) + " " })
	return strings.ReplaceAll(text, "%", " percent ")
}

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

func normalizeChineseText(text string) string {
	text = norm.NFKC.String(text)
	text = groupedNumberPattern.ReplaceAllStringFunc(text, func(s string) string { return strings.ReplaceAll(s, ",", "") })
	return numberPattern.ReplaceAllStringFunc(text, func(s string) string {
		integer, fraction, decimal := strings.Cut(s, ".")
		n, err := strconv.ParseUint(integer, 10, 64)
		// Years and long identifiers are read digit by digit.
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
