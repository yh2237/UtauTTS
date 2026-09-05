package tts

import (
	"math"
	"strings"

	"utautts/internal/frontend"
	"utautts/internal/prosody"
	"utautts/internal/render"
)

// A conservative fallback for English, whose phones cannot use the Japanese
// accent model. Explicit lexical stress is used when the pronunciation has it.
// This is a rule-based speech baseline, not a trained English prosody model.
func englishPredictions(morae []frontend.Mora) []prosody.Prediction {
	result := make([]prosody.Prediction, len(morae))
	for i, mora := range morae {
		factor := 1.0
		if mora.Vowel != "" && !mora.Pause {
			switch mora.Stress {
			case 1:
				factor = 1.2
			case 2:
				factor = 1.1
			}
			// Reserve time for codas in syllable-based phonemizers.
			if mora.DurationScale == 0 && mora.Aliases != nil {
				factor += math.Min(0.5, float64(len(mora.Aliases.Endings))*0.15)
			}
		}
		result[i] = prosody.Prediction{DurationFactor: factor, EnergyFactor: 1, PitchFactor: 1}
	}
	return result
}

func englishSpeechCurve(morae []frontend.Mora, timings []prosody.MoraTiming, durationMS float64, text string) *render.PitchCurve {
	if len(morae) == 0 || len(morae) != len(timings) || durationMS <= 0 {
		return nil
	}
	curve := &render.PitchCurve{FrameMS: 10, Cents: make([]float64, int(math.Ceil(durationMS/10))+1)}
	for start := 0; start < len(morae); {
		if morae[start].Pause {
			start++
			continue
		}
		end := start
		for end+1 < len(morae) && !morae[end+1].Pause {
			end++
		}
		left := timings[start].StartMS
		right := timings[end].StartMS + timings[end].DurationMS
		for i := start; i <= end; i++ {
			timing := timings[i]
			for frame := max(0, int(math.Ceil(timing.StartMS/10))); frame < len(curve.Cents) && float64(frame)*10 <= timing.StartMS+timing.DurationMS; frame++ {
				t := float64(frame) * 10
				phase := (t - left) / math.Max(1, right-left)
				cents := 55 - 110*phase
				if morae[i].Vowel != "" && morae[i].Stress > 0 {
					local := (t - timing.StartMS) / math.Max(1, timing.DurationMS)
					cents += 65 * math.Sin(math.Pi*local) / float64(morae[i].Stress)
				}
				if strings.HasSuffix(strings.TrimSpace(text), "?") && end >= len(morae)-2 && phase > 0.7 {
					cents += 140 * (phase - 0.7) / 0.3
				}
				curve.Cents[frame] = cents
			}
		}
		start = end + 1
	}
	return render.ConstrainPitchCurve(curve, 20, 8)
}
