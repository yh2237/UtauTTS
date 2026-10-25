package g2p

import (
	"regexp"
	"strings"
)

var (
	esCurrencyPrefix = regexp.MustCompile(`(?i)(US\$|€|\$|£)\s*([0-9]+(?:[.,][0-9]+)*)`)
	esCurrencySuffix = regexp.MustCompile(`(?i)([0-9]+(?:[.,][0-9]+)*)\s*(US\$|€|\$|£)`)
	esDecimalPattern = regexp.MustCompile(`([0-9]+),([0-9]+)`)
	esPercentPattern = regexp.MustCompile(`([0-9]+(?:[.,][0-9]+)*)\s*%`)
)

// normalizeSpanishは、スペイン語の通貨記号・小数・割合を語にする。数字そのものは
// frontendのspanishNumberWordsが読むので、ここでは記号と小数の語だけを足す。
func normalizeSpanish(text string) string {
	text = esCurrencyPrefix.ReplaceAllStringFunc(text, func(match string) string {
		parts := esCurrencyPrefix.FindStringSubmatch(match)
		return spanishCurrencyWords(parts[1], parts[2])
	})
	text = esCurrencySuffix.ReplaceAllStringFunc(text, func(match string) string {
		parts := esCurrencySuffix.FindStringSubmatch(match)
		return spanishCurrencyWords(parts[2], parts[1])
	})
	text = esDecimalPattern.ReplaceAllString(text, "$1 coma $2")
	text = esPercentPattern.ReplaceAllStringFunc(text, func(match string) string {
		parts := esPercentPattern.FindStringSubmatch(match)
		return parts[1] + " por ciento"
	})
	return text
}

func spanishCurrencyWords(symbol, amount string) string {
	integer, fraction, _ := splitSpanishAmount(amount)
	singular, plural, centsName := "euro", "euros", "céntimos"
	switch strings.ToUpper(symbol) {
	case "US$", "$":
		singular, plural, centsName = "dólar", "dólares", "centavos"
	case "£":
		singular, plural, centsName = "libra", "libras", "peniques"
	}
	if integer == "" || strings.TrimLeft(integer, "0") == "" {
		if cents := spanishCents(fraction); cents != "" {
			return cents + " " + centsName
		}
		return "cero " + plural
	}
	words := integer + " "
	if spanishAmountIsOne(amount) {
		words += singular
	} else {
		words += plural
	}
	if cents := spanishCents(fraction); cents != "" {
		words += " con " + cents + " " + centsName
	}
	return words
}

func spanishCents(fraction string) string {
	if fraction == "" {
		return ""
	}
	if len(fraction) == 1 {
		fraction += "0"
	}
	if len(fraction) > 2 {
		fraction = fraction[:2]
	}
	return strings.TrimLeft(fraction, "0")
}

func spanishAmountIsOne(amount string) bool {
	integer, fraction, _ := splitSpanishAmount(amount)
	if strings.TrimLeft(integer, "0") != "1" {
		return false
	}
	return strings.Trim(fraction, "0") == ""
}

func splitSpanishAmount(token string) (integer, fraction string, decimal bool) {
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
