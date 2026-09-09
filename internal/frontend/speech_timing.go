package frontend

import "strings"

func englishSyllablePhones(s englishSyllable) []Phone {
	var phones []Phone
	for _, symbol := range s.onset {
		phones = append(phones, Phone{symbol, "onset"})
	}
	phones = append(phones, Phone{s.vowel, "nucleus"})
	for _, symbol := range s.coda {
		phones = append(phones, Phone{symbol, "coda"})
	}
	return phones
}

// PhoneWeight is a conservative duration prior, not an acoustic boundary estimate.
// Keep it independent of CV/VCV/diphone aliases so all planners share the timing.
func PhoneWeight(symbol, role string) float64 {
	if role == "nucleus" {
		return 1
	}
	switch strings.ToLower(symbol) {
	case "s", "sh", "f", "th", "z", "zh", "v", "dh", "ts":
		return 0.65
	case "ch", "jh":
		return 0.55
	case "p", "t", "k", "b", "d", "g", "dx":
		return 0.35
	case "m", "n", "ng", "l", "r":
		return 0.5
	default:
		return 0.45
	}
}

// PhoneSpans divides a known duration; no source audio is inferred here.
func PhoneSpans(phones []Phone, duration float64) []float64 {
	spans := make([]float64, len(phones))
	sum := 0.0
	for i, p := range phones {
		spans[i] = PhoneWeight(p.Symbol, p.Role)
		sum += spans[i]
	}
	if sum > 0 {
		for i := range spans {
			spans[i] *= duration / sum
		}
	}
	return spans
}
