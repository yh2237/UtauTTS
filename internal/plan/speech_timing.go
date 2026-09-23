package plan

import (
	"math"
	"strings"

	"utautts/internal/frontend"
)

// PhoneTimingは合成目標であり音響解析の結果ではない。
type PhoneTiming struct {
	Position   int     `json:"position"`
	Symbol     string  `json:"symbol"`
	Role       string  `json:"role"`
	StartMS    float64 `json:"start_ms"`
	DurationMS float64 `json:"duration_ms"`
}

// 英語codaの最低長。破裂音は閉鎖+解放を知覚できる長さを確保する。
const (
	englishCodaMinStopMS       = 65.0
	englishCodaMinContinuantMS = 55.0
	// codaへ再配分した後も母音に残す最低長。
	englishCodaMinVowelMS = 45.0
	// 過剰な伸長を防ぐcoda上限。
	englishCodaMaxMS = 85.0
)

func speechEndingTiming(mora frontend.Mora, spans []float64, index int, start, duration float64) (float64, float64, float64) {
	if len(spans) != len(mora.Phones) {
		spans = frontend.PhoneSpans(mora.Phones, duration)
	}
	codaStart := start
	var codaSpans []float64
	var codaPhones []frontend.Phone
	for i, p := range mora.Phones {
		if p.Role == "coda" {
			codaSpans = append(codaSpans, spans[i])
			codaPhones = append(codaPhones, p)
		} else {
			codaStart += spans[i]
		}
	}
	if len(codaSpans) == 0 {
		return start + duration - endingDurationFor(duration, 1), endingDurationFor(duration, 1), 0
	}
	codaStart, codaSpans, floorMS := englishCodaFloor(mora, codaPhones, codaStart, codaSpans, start, duration)
	if index == 0 {
		return codaStart, codaSpans[0], floorMS
	}
	rest := 0.0
	for _, span := range codaSpans[1:] {
		rest += span
	}
	if rest == 0 {
		rest = math.Min(codaSpans[0]*0.5, 20)
		return codaStart + codaSpans[0] - rest, rest, floorMS
	}
	return codaStart + codaSpans[0], rest, floorMS
}

// englishCodaFloorは英語の語末子音が短すぎるとき、先行母音から再配分して
// 最低長を確保する。モーラ総長は変えない。適用したcoda長を返す。
func englishCodaFloor(mora frontend.Mora, codaPhones []frontend.Phone, codaStart float64, codaSpans []float64, start, duration float64) (float64, []float64, float64) {
	if mora.Language != frontend.LanguageEnglish {
		return codaStart, codaSpans, 0
	}
	floor := englishCodaFloorMS(codaPhones)
	total := 0.0
	for _, span := range codaSpans {
		total += span
	}
	if floor <= 0 || total <= 0 || total >= floor {
		return codaStart, codaSpans, 0
	}
	// 母音の最低長とcoda上限の両方を守る。
	maximum := math.Min(duration-englishCodaMinVowelMS, englishCodaMaxMS)
	if maximum <= total {
		return codaStart, codaSpans, 0
	}
	target := math.Min(floor, maximum)
	scaled := make([]float64, len(codaSpans))
	for i, span := range codaSpans {
		scaled[i] = span * target / total
	}
	return start + duration - target, scaled, target
}

// englishCodaFloorMSはcoda音素クラスごとの最低長を返す。対象外は0。
func englishCodaFloorMS(phones []frontend.Phone) float64 {
	stop, continuant := false, false
	for _, phone := range phones {
		switch strings.ToLower(strings.TrimSpace(phone.Symbol)) {
		case "p", "t", "k", "b", "d", "g", "q", "ch", "jh":
			stop = true
		case "s", "sh", "f", "th", "z", "zh", "v", "dh", "m", "n", "ng", "l", "r":
			continuant = true
		}
	}
	switch {
	case stop:
		return englishCodaMinStopMS
	case continuant:
		return englishCodaMinContinuantMS
	default:
		return 0
	}
}
