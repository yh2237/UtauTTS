package tts

import (
	"fmt"
	"math"
	"strings"
	"utautts/internal/frontend"
	"utautts/internal/prosody"
	"utautts/internal/render"
)

func experimentalSpeechTiming(cfg Config) bool {
	return cfg.SpeechProsodyExperiment == "timing" || cfg.SpeechProsodyExperiment == "both"
}
func experimentalSpeechPitch(cfg Config) bool {
	return cfg.SpeechProsodyExperiment == "pitch" || cfg.SpeechProsodyExperiment == "both"
}

func validateSpeechExperiment(cfg Config) error {
	switch cfg.SpeechProsodyExperiment {
	case "", "baseline":
		return nil
	case "timing", "pitch", "both":
	default:
		return fmt.Errorf("unknown speech prosody experiment %q", cfg.SpeechProsodyExperiment)
	}
	return validateMultilingualWorldExperiment(cfg)
}

func validateMultilingualWorldExperiment(cfg Config) error {
	_, phonemizer, err := frontend.ResolveLanguage(cfg.Language, cfg.Phonemizer)
	if err != nil {
		return err
	}
	if phonemizer != frontend.PhonemizerEnglishDelta && phonemizer != frontend.PhonemizerEnglishVCCV && phonemizer != frontend.PhonemizerChinese {
		return fmt.Errorf("experiment requires en-delta, en-vccv or zh-cvvc")
	}
	if cfg.Renderer != "utautts-world-phrase" {
		return fmt.Errorf("experiment requires utautts-world-phrase")
	}
	return nil
}

// 句の長さを保ちながら手動指定以外の音素長を配分する。
func speechRhythmExperiment(morae []frontend.Mora, base []prosody.Prediction, manual []float64) []prosody.Prediction {
	result := append([]prosody.Prediction(nil), base...)
	for start := 0; start < len(morae); {
		if morae[start].Pause {
			start++
			continue
		}
		end := start
		for end < len(morae) && !morae[end].Pause {
			end++
		}
		oldSum, newSum := 0.0, 0.0
		for i := start; i < end; i++ {
			if i < len(manual) && manual[i] > 0 {
				continue
			}
			m := morae[i]
			factor := 1.0
			if m.Language == frontend.LanguageEnglish {
				switch {
				case m.Stress == 1:
					factor = 1.35
				case m.Stress == 2:
					factor = 1.12
				case m.StressKnown:
					factor = .65
				}
				for _, p := range m.Phones {
					if p.Role != "nucleus" {
						factor += .18 * frontend.PhoneWeight(p.Symbol, p.Role)
					}
				}
			} else {
				switch m.Tone {
				case 2, 3:
					factor = 1.12
				case 4:
					factor = .94
				case 5:
					factor = .55
				}
			}
			if i == end-1 {
				factor *= 1.15
			}
			scale := previewDurationFor(m, 1)
			oldSum += base[i].DurationFactor * scale
			newSum += factor * scale
			result[i].DurationFactor = factor
		}
		if newSum > 0 {
			for i := start; i < end; i++ {
				if i >= len(manual) || manual[i] <= 0 {
					result[i].DurationFactor *= oldSum / newSum
				}
			}
		}
		start = end
	}
	return result
}

func speechPitchExperiment(language string, morae []frontend.Mora, timings []prosody.MoraTiming, duration float64, text string, strength float64) *render.PitchCurve {
	if language == frontend.LanguageChinese {
		return mandarinToneCurveAligned(morae, timings, duration, true)
	}
	if len(morae) == 0 || len(morae) != len(timings) || duration <= 0 {
		return nil
	}
	curve := &render.PitchCurve{FrameMS: 10, Cents: make([]float64, int(math.Ceil(duration/10))+1)}
	for start := 0; start < len(morae); {
		if morae[start].Pause {
			start++
			continue
		}
		end := start
		for end+1 < len(morae) && !morae[end+1].Pause {
			end++
		}
		left, right := timings[start].StartMS, timings[end].StartMS+timings[end].DurationMS
		nuclear := -1
		for i := start; i <= end; i++ {
			if morae[i].Stress == 1 {
				nuclear = i
			}
		}
		for i := start; i <= end; i++ {
			tm := timings[i]
			// アクセントを語末子音まで延ばさず母音区間に収める。
			weights, coda := 1.0, 0.0
			for _, p := range morae[i].Phones {
				if p.Role == "coda" {
					coda += frontend.PhoneWeight(p.Symbol, p.Role)
				}
			}
			weights += coda
			vowelMS := tm.DurationMS / weights
			for frame := max(0, int(math.Ceil(tm.StartMS/10))); frame < len(curve.Cents) && float64(frame)*10 < tm.StartMS+tm.DurationMS; frame++ {
				t := float64(frame) * 10
				phase := (t - left) / math.Max(1, right-left)
				value := 80 - 160*phase
				local := (t - tm.StartMS) / math.Max(1, vowelMS)
				if local < 1 && morae[i].Stress > 0 {
					accent := 65.0
					if i == nuclear {
						accent = 120
					}
					value += accent * math.Sin(math.Pi*local) / float64(morae[i].Stress)
				}
				if strings.HasSuffix(strings.TrimSpace(text), "?") && end >= len(morae)-2 && phase > .7 {
					x := (phase - .7) / .3
					value += 200 * x * x * (3 - 2*x)
				}
				curve.Cents[frame] = value
			}
		}
		start = end + 1
	}
	return scaleAutomaticPitchCurve(render.ConstrainPitchCurve(curve, 20, 8), strength)
}
