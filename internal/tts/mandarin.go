package tts

import (
	"math"
	"strings"

	"utautts/internal/frontend"
	"utautts/internal/prosody"
	"utautts/internal/render"
)

const mandarinPitchFrameMS = 10

type tonePoint struct {
	position float64
	cents    float64
}

func mandarinToneCurve(morae []frontend.Mora, timings []prosody.MoraTiming, durationMS float64) *render.PitchCurve {
	if len(morae) == 0 || len(timings) != len(morae) || durationMS <= 0 {
		return nil
	}
	hasTone := false
	tones := mandarinSurfaceTones(morae)
	for _, mora := range morae {
		if mora.Tone >= 1 && mora.Tone <= 5 {
			hasTone = true
		}
	}
	if !hasTone {
		return nil
	}

	curve := &render.PitchCurve{
		FrameMS: mandarinPitchFrameMS,
		Cents:   make([]float64, int(math.Ceil(durationMS/mandarinPitchFrameMS))+1),
	}
	for i, timing := range timings {
		if morae[i].Pause || timing.DurationMS <= 0 || tones[i] < 1 || tones[i] > 5 {
			continue
		}
		phraseFinal := i+1 == len(morae) || morae[i+1].Pause
		points := mandarinTonePoints(tones[i], phraseFinal)
		if tones[i] == 5 && i > 0 && !morae[i-1].Pause {
			end := map[int]float64{1: -100, 2: -55, 3: 65, 4: -130}[tones[i-1]]
			points = []tonePoint{{0, end + 25}, {1, end}}
		}
		// The lexical tone belongs to the syllable nucleus, not its unvoiced onset.
		onset := 0.0
		spans := frontend.PhoneSpans(morae[i].Phones, timing.DurationMS)
		for j, p := range morae[i].Phones {
			if p.Role == "onset" {
				onset += spans[j]
			}
		}
		start := max(0, int(math.Ceil(timing.StartMS/mandarinPitchFrameMS)))
		end := min(len(curve.Cents)-1, int(math.Floor((timing.StartMS+timing.DurationMS)/mandarinPitchFrameMS)))
		for frame := start; frame <= end; frame++ {
			position := (float64(frame)*mandarinPitchFrameMS - timing.StartMS - onset) / math.Max(1, timing.DurationMS-onset)
			curve.Cents[frame] = interpolateTone(points, max(0, min(1, position)))
		}
	}
	return curve
}

// Apply sandhi inside lexical words first, then across adjacent words. Lexical
// tones remain in the frontend record so diagnostics can show both versions.
func mandarinSurfaceTones(morae []frontend.Mora) []int {
	tones := make([]int, len(morae))
	for i, m := range morae {
		tones[i] = m.Tone
	}
	for i, m := range morae {
		if m.Pause || i+1 == len(morae) || morae[i+1].Pause {
			continue
		}
		next := morae[i+1].Tone
		if m.SourceText == "不" && m.Tone == 4 && next == 4 {
			tones[i] = 2
		}
		if m.SourceText == "一" && m.Tone == 1 && !(i > 0 && morae[i-1].SourceText == "第") {
			// Counting sequences and years retain yi1.
			if i+1 < len(morae) && strings.Contains("零一二三四五六七八九十", morae[i+1].SourceText) && morae[i+1].SourceText != "" {
				continue
			}
			if next == 4 {
				tones[i] = 2
			} else if next >= 1 && next <= 3 {
				tones[i] = 4
			}
		}
	}
	for i := 0; i+1 < len(morae); i++ {
		if !morae[i].Pause && !morae[i+1].Pause && !morae[i].WordEnd && morae[i].WordIndex == morae[i+1].WordIndex && tones[i] == 3 && tones[i+1] == 3 {
			tones[i] = 2
		}
	}
	for i := len(morae) - 2; i >= 0; i-- {
		if !morae[i].Pause && !morae[i+1].Pause && tones[i] == 3 && tones[i+1] == 3 {
			tones[i] = 2
		}
	}
	return tones
}

func mandarinPredictions(morae []frontend.Mora) []prosody.Prediction {
	result := make([]prosody.Prediction, len(morae))
	for i, m := range morae {
		factor := 1.0
		if m.Tone == 5 {
			factor = 0.65
		}
		if !m.Pause && (i+1 == len(morae) || morae[i+1].Pause) {
			factor *= 1.12
		}
		result[i] = prosody.Prediction{DurationFactor: factor, PitchFactor: 1, EnergyFactor: 1}
	}
	return result
}

func mandarinTonePoints(tone int, phraseFinal bool) []tonePoint {
	switch tone {
	case 1:
		return []tonePoint{{0, 145}, {1, 145}}
	case 2:
		return []tonePoint{{0, -20}, {0.3, -55}, {1, 145}}
	case 3:
		if phraseFinal {
			return []tonePoint{{0, -35}, {0.55, -145}, {1, 65}}
		}
		return []tonePoint{{0, -35}, {0.65, -145}, {1, -115}}
	case 4:
		return []tonePoint{{0, 145}, {0.2, 115}, {1, -145}}
	case 5:
		return []tonePoint{{0, -10}, {1, -35}}
	default:
		return []tonePoint{{0, 0}, {1, 0}}
	}
}

func interpolateTone(points []tonePoint, position float64) float64 {
	for i := 1; i < len(points); i++ {
		if position <= points[i].position {
			left, right := points[i-1], points[i]
			span := right.position - left.position
			if span <= 0 {
				return right.cents
			}
			ratio := (position - left.position) / span
			ratio = ratio * ratio * (3 - 2*ratio)
			return left.cents + (right.cents-left.cents)*ratio
		}
	}
	return points[len(points)-1].cents
}
