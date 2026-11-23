package en

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var (
	enSmall              = strings.Fields("zero one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen sixteen seventeen eighteen nineteen")
	enTens               = strings.Fields("zero ten twenty thirty forty fifty sixty seventy eighty ninety")
	numberPattern        = regexp.MustCompile(`[0-9]+(?:\.[0-9]+)?`)
	timePattern          = regexp.MustCompile(`\b([0-9]{1,2}):([0-9]{2})\b`)
	groupedNumberPattern = regexp.MustCompile(`\b[0-9]{1,3}(?:,[0-9]{3})+\b`)
)

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

// Normalizeは英語の時刻・数字・割合を語へ広げる。
func Normalize(text string) string {
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
		value := EnglishNumber(parts[0])
		if minute == 0 {
			return value + " o'clock"
		}
		if minute < 10 {
			return value + " oh " + EnglishCardinal(uint64(minute))
		}
		return value + " " + EnglishCardinal(uint64(minute))
	})
	text = numberPattern.ReplaceAllStringFunc(text, func(s string) string { return " " + EnglishNumber(s) + " " })
	return strings.ReplaceAll(text, "%", " percent ")
}

// Wordsは英語の読みを作るときと同じ語の並びを返す。区切りは"<pause>"になる。
func Words(text string) []string {
	return latinWords(Normalize(text))
}

func latinWords(text string) []string {
	var result []string
	var word strings.Builder
	flush := func() {
		if word.Len() > 0 {
			result = append(result, word.String())
			word.Reset()
		}
	}
	for _, r := range text {
		if unicode.IsLetter(r) || r == '\'' {
			word.WriteRune(r)
			continue
		}
		flush()
		if strings.ContainsRune(",.;:!?、。！？\n", r) && len(result) > 0 && result[len(result)-1] != "<pause>" {
			result = append(result, "<pause>")
		}
	}
	flush()
	return result
}
