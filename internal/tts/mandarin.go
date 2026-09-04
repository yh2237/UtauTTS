package tts

import (
	"math"

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
	tones := make([]int, len(morae))
	for i, mora := range morae {
		tones[i] = mora.Tone
		if mora.Tone >= 1 && mora.Tone <= 5 {
			hasTone = true
		}
	}
	if !hasTone {
		return nil
	}
	for i := 0; i+1 < len(tones); i++ {
		if !morae[i].Pause && !morae[i+1].Pause && tones[i] == 3 && tones[i+1] == 3 {
			tones[i] = 2
		}
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
		start := max(0, int(math.Ceil(timing.StartMS/mandarinPitchFrameMS)))
		end := min(len(curve.Cents)-1, int(math.Floor((timing.StartMS+timing.DurationMS)/mandarinPitchFrameMS)))
		for frame := start; frame <= end; frame++ {
			position := (float64(frame)*mandarinPitchFrameMS - timing.StartMS) / timing.DurationMS
			curve.Cents[frame] = interpolateTone(points, max(0, min(1, position)))
		}
	}
	return curve
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
