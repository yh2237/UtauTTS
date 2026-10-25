package g2p

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	ptCurrencyPattern = regexp.MustCompile(`(?i)(US\$|R\$|€|£)\s*(-?[0-9]+(?:[.,][0-9]+)*)`)
	ptPercentPattern  = regexp.MustCompile(`(-?[0-9]+(?:[.,][0-9]+)*)\s*%`)
	ptNumberPattern   = regexp.MustCompile(`-?\b[0-9]+(?:[.,][0-9]+)*\b`)
)

var ptUnits = []string{
	"zero", "um", "dois", "três", "quatro", "cinco", "seis", "sete", "oito", "nove",
	"dez", "onze", "doze", "treze", "catorze", "quinze", "dezesseis", "dezessete", "dezoito", "dezenove",
}

var ptTens = []string{"", "", "vinte", "trinta", "quarenta", "cinquenta", "sessenta", "setenta", "oitenta", "noventa"}

var ptHundreds = []string{"", "cento", "duzentos", "trezentos", "quatrocentos", "quinhentos",
	"seiscentos", "setecentos", "oitocentos", "novecentos"}

// normalizePortugueseは、ポルトガル語（ブラジル）の数字・通貨・割合を語にする。
func normalizePortuguese(text string) string {
	text = ptCurrencyPattern.ReplaceAllStringFunc(text, func(match string) string {
		parts := ptCurrencyPattern.FindStringSubmatch(match)
		return ptCurrencyWords(parts[1], parts[2])
	})
	text = ptPercentPattern.ReplaceAllStringFunc(text, func(match string) string {
		parts := ptPercentPattern.FindStringSubmatch(match)
		return ptPercentWords(parts[1])
	})
	text = ptNumberPattern.ReplaceAllStringFunc(text, ptNumberTokenWords)
	return text
}

func ptNumberTokenWords(token string) string {
	negative := strings.HasPrefix(token, "-")
	token = strings.TrimPrefix(token, "-")
	integer, fraction, decimal := splitPtNumber(token)
	words := ptNumberWords(integer)
	if decimal && fraction != "" {
		words += " vírgula " + ptFractionWords(fraction)
	}
	if negative {
		words = "menos " + words
	}
	return words
}

func ptPercentWords(token string) string {
	integer, fraction, decimal := splitPtNumber(token)
	words := ptNumberWords(integer)
	if decimal && fraction != "" {
		words += " vírgula " + ptFractionWords(fraction)
	}
	return words + " por cento"
}

func ptCurrencyWords(symbol, amount string) string {
	negative := strings.HasPrefix(amount, "-")
	amount = strings.TrimPrefix(amount, "-")
	integer, fraction, _ := splitPtNumber(amount)
	singular, plural := "real", "reais"
	switch strings.ToUpper(symbol) {
	case "US$":
		singular, plural = "dólar", "dólares"
	case "€":
		singular, plural = "euro", "euros"
	case "£":
		singular, plural = "libra", "libras"
	}
	var words string
	if integer != "0" || fraction == "" {
		words = ptNumberWords(integer)
		if isPtOne(integer) {
			words += " " + singular
		} else {
			words += " " + plural
		}
	}
	if cents := ptCentsWords(fraction); cents != "" {
		if words != "" {
			words += " e "
		}
		words += cents
	}
	if words == "" {
		words = "zero " + plural
	}
	if negative {
		words = "menos " + words
	}
	return words
}

func isPtOne(integer string) bool {
	return strings.TrimLeft(integer, "0") == "1"
}

func ptCentsWords(fraction string) string {
	if fraction == "" {
		return ""
	}
	if len(fraction) == 1 {
		fraction += "0"
	}
	value := strings.TrimLeft(fraction, "0")
	if value == "" {
		return ""
	}
	return ptNumberWords(value) + " centavos"
}

// splitPtNumberは数字を整数部と小数部へ分ける。ブラジル式の「1.234,56」と
// 小数点だけの「3.5」の両方を受ける。
func splitPtNumber(token string) (integer, fraction string, decimal bool) {
	token = strings.TrimSpace(token)
	switch {
	case strings.Contains(token, ","):
		parts := strings.SplitN(token, ",", 2)
		return strings.ReplaceAll(parts[0], ".", ""), parts[1], true
	case strings.Count(token, ".") > 1:
		return strings.ReplaceAll(token, ".", ""), "", false
	case strings.Contains(token, "."):
		parts := strings.SplitN(token, ".", 2)
		if len(parts[1]) == 3 && len(parts[0]) <= 3 {
			return parts[0] + parts[1], "", false
		}
		return parts[0], parts[1], true
	default:
		return token, "", false
	}
}

func ptFractionWords(fraction string) string {
	fraction = strings.TrimLeft(fraction, "0")
	if fraction == "" {
		return "zero"
	}
	return ptNumberWords(fraction)
}

func ptNumberWords(digits string) string {
	digits = strings.TrimLeft(digits, "0")
	if digits == "" {
		return "zero"
	}
	value, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || value > 999999999999 {
		return ptDigitWords(digits)
	}
	return ptBelowBillion(value)
}

func ptDigitWords(digits string) string {
	var words []string
	for _, r := range digits {
		if r >= '0' && r <= '9' {
			words = append(words, ptUnits[r-'0'])
		}
	}
	return strings.Join(words, " ")
}

func ptBelowBillion(n int64) string {
	if n < 1000 {
		return ptBelowThousand(int(n))
	}
	var words []string
	if billions := n / 1000000000; billions > 0 {
		words = append(words, ptLargeGroup(billions, "bilhão", "bilhões"))
		n %= 1000000000
	}
	if millions := n / 1000000; millions > 0 {
		words = append(words, ptLargeGroup(millions, "milhão", "milhões"))
		n %= 1000000
	}
	if thousands := n / 1000; thousands > 0 {
		if thousands == 1 {
			words = append(words, "mil")
		} else {
			words = append(words, ptBelowThousand(int(thousands)), "mil")
		}
		n %= 1000
	}
	if n > 0 {
		if len(words) > 0 && (n < 100 || n%100 == 0) {
			words = append(words, "e")
		}
		words = append(words, ptBelowThousand(int(n)))
	}
	return strings.Join(words, " ")
}

func ptLargeGroup(value int64, singular, plural string) string {
	if value == 1 {
		return "um " + singular
	}
	return ptBelowThousand(int(value)) + " " + plural
}

func ptBelowThousand(n int) string {
	if n == 100 {
		return "cem"
	}
	if n < 100 {
		return ptBelowHundred(n)
	}
	words := ptHundreds[n/100]
	if rest := n % 100; rest > 0 {
		words += " e " + ptBelowHundred(rest)
	}
	return words
}

func ptBelowHundred(n int) string {
	if n < 20 {
		return ptUnits[n]
	}
	words := ptTens[n/10]
	if n%10 > 0 {
		words += " e " + ptUnits[n%10]
	}
	return words
}
