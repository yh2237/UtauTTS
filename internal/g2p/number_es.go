package g2p

import (
	"strconv"
	"strings"
)

var esUnits = []string{"cero", "uno", "dos", "tres", "cuatro", "cinco", "seis", "siete", "ocho", "nueve",
	"diez", "once", "doce", "trece", "catorce", "quince", "dieciséis", "diecisiete", "dieciocho", "diecinueve",
	"veinte", "veintiuno", "veintidós", "veintitrés", "veinticuatro", "veinticinco", "veintiséis", "veintisiete",
	"veintiocho", "veintinueve"}

var esTens = []string{"", "", "", "treinta", "cuarenta", "cincuenta", "sesenta", "setenta", "ochenta", "noventa"}

var esHundreds = []string{"", "ciento", "doscientos", "trescientos", "cuatrocientos", "quinientos",
	"seiscientos", "setecientos", "ochocientos", "novecientos"}

// SpanishNumberWordsは数字の並びをスペイン語の語にする。数字以外の文字が混じる語は、
// 数字の部分だけを読む。999,999,999までを読み、それより大きい数は1桁ずつ読む。
func SpanishNumberWords(token string) string {
	var parts []string
	var digits, letters strings.Builder
	flush := func() {
		if digits.Len() > 0 {
			parts = append(parts, esNumber(digits.String()))
			digits.Reset()
		}
		if letters.Len() > 0 {
			parts = append(parts, letters.String())
			letters.Reset()
		}
	}
	for _, r := range token {
		if r >= '0' && r <= '9' {
			if letters.Len() > 0 {
				flush()
			}
			digits.WriteRune(r)
		} else {
			if digits.Len() > 0 {
				flush()
			}
			letters.WriteRune(r)
		}
	}
	flush()
	return strings.Join(parts, " ")
}

func esNumber(digits string) string {
	value, err := strconv.Atoi(digits)
	if err != nil || len(digits) > 9 {
		var words []string
		for _, r := range digits {
			words = append(words, esUnits[r-'0'])
		}
		return strings.Join(words, " ")
	}
	return esBelowBillion(value)
}

func esBelowBillion(n int) string {
	if n < 1000 {
		return esBelowThousand(n)
	}
	var words []string
	if millions := n / 1000000; millions > 0 {
		if millions == 1 {
			words = append(words, "un millón")
		} else {
			words = append(words, esApocope(esBelowThousand(millions)), "millones")
		}
		n %= 1000000
	}
	if thousands := n / 1000; thousands > 0 {
		if thousands > 1 {
			words = append(words, esApocope(esBelowThousand(thousands)))
		}
		words = append(words, "mil")
		n %= 1000
	}
	if n > 0 {
		words = append(words, esBelowThousand(n))
	}
	return strings.Join(words, " ")
}

func esBelowThousand(n int) string {
	if n == 100 {
		return "cien"
	}
	var words []string
	if hundreds := n / 100; hundreds > 0 {
		words = append(words, esHundreds[hundreds])
		n %= 100
		if n == 0 {
			return strings.Join(words, " ")
		}
	}
	switch {
	case n < 30:
		if n > 0 || len(words) == 0 {
			words = append(words, esUnits[n])
		}
	default:
		words = append(words, esTens[n/10])
		if n%10 > 0 {
			words = append(words, "y", esUnits[n%10])
		}
	}
	return strings.Join(words, " ")
}

// esApocopeは、千や百万の前のunoをunにする（veintiuno mil → veintiún mil）。
func esApocope(words string) string {
	switch {
	case words == "uno":
		return "un"
	case strings.HasSuffix(words, "veintiuno"):
		return strings.TrimSuffix(words, "veintiuno") + "veintiún"
	case strings.HasSuffix(words, " uno"):
		return strings.TrimSuffix(words, "uno") + "un"
	}
	return words
}
