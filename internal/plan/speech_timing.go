package plan

import (
	"math"
	"utautts/internal/frontend"
)

// PhoneTiming is a target, never an acoustic measurement or forced alignment.
type PhoneTiming struct {
	Position   int     `json:"position"`
	Symbol     string  `json:"symbol"`
	Role       string  `json:"role"`
	StartMS    float64 `json:"start_ms"`
	DurationMS float64 `json:"duration_ms"`
}

func speechEndingTiming(mora frontend.Mora, index int, start, duration float64) (float64, float64) {
	spans := frontend.PhoneSpans(mora.Phones, duration)
	codaStart := start
	var codaSpans []float64
	for i, p := range mora.Phones {
		if p.Role == "coda" {
			codaSpans = append(codaSpans, spans[i])
		} else {
			codaStart += spans[i]
		}
	}
	if len(codaSpans) == 0 {
		return start + duration - endingDurationFor(duration, 1), endingDurationFor(duration, 1)
	}
	if index == 0 {
		return codaStart, codaSpans[0]
	}
	rest := 0.0
	for _, span := range codaSpans[1:] {
		rest += span
	}
	if rest == 0 {
		rest = math.Min(codaSpans[0]*0.5, 20)
		return codaStart + codaSpans[0] - rest, rest
	}
	return codaStart + codaSpans[0], rest
}
