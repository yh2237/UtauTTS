package tts

import (
	"fmt"
	"math"
	"strings"

	"utautts/internal/frontend"
	"utautts/internal/plan"
	"utautts/internal/prosody"
	"utautts/internal/render"
)

type chineseProfile struct{}

func (chineseProfile) Language() string { return frontend.LanguageChinese }

func (chineseProfile) ParsePronunciation(cfg Config, phonemizer string) (string, []frontend.Mora, error) {
	if phonemizer != frontend.PhonemizerChinese {
		return "", nil, fmt.Errorf("unsupported phonemizer %q for language %q", phonemizer, frontend.LanguageChinese)
	}
	var presamp frontend.PresampConfig
	if cfg.Voicebank != nil {
		presamp = cfg.Voicebank.Presamp.FrontendConfig()
	}
	return frontend.ParseChineseCVVCWithConfig(cfg.Text, cfg.Reading, cfg.Dictionary, presamp)
}

func (chineseProfile) ApplySpeechProfile(*Config) {}

func (chineseProfile) ProsodyModelFallback(configuredPath string) string {
	return mandarinFallbackProsodyModelPath(configuredPath)
}

func (chineseProfile) SupportsStretchAdapt() bool { return false }

func (chineseProfile) PhoneTiming(cfg Config, morae []frontend.Mora, _ bool) ([][]float64, string) {
	return speechDurationsForConfig(cfg, morae), "multilingual-speech-score-v1"
}

func (chineseProfile) Predict(morae []frontend.Mora) []prosody.Prediction {
	return mandarinPredictions(morae)
}

func (chineseProfile) AdjustPredictions(cfg Config, _ *prosody.Model, morae []frontend.Mora, predictions []prosody.Prediction, _ []prosody.FeatureFrame) []prosody.Prediction {
	return applySpeechScore(cfg, morae, predictions)
}

func (chineseProfile) AutomaticPitchCurve(cfg Config, model *prosody.Model, morae []frontend.Mora, timings []prosody.MoraTiming, durationMS float64) (*render.PitchCurve, bool) {
	curve := mandarinToneCurve(morae, timings, durationMS)
	if model != nil && model.MandarinIntonation != nil {
		curve = applyMandarinIntonation(curve, model.MandarinIntonation, morae, timings)
	}
	curve = learnedSpeechCurve(cfg, morae, timings, curve)
	return curve, curve != nil
}

func applyMandarinIntonation(curve *render.PitchCurve, model *prosody.MandarinIntonationModel, morae []frontend.Mora, timings []prosody.MoraTiming) *render.PitchCurve {
	if curve == nil || model == nil || len(morae) != len(timings) {
		return curve
	}
	result := &render.PitchCurve{FrameMS: curve.FrameMS, Cents: append([]float64(nil), curve.Cents...)}
	tones := mandarinSurfaceTones(morae)
	count := 0
	for _, mora := range morae {
		if !mora.Pause {
			count++
		}
	}
	position := 0
	for index, mora := range morae {
		if mora.Pause || timings[index].DurationMS <= 0 || tones[index] < 1 || tones[index] > 5 {
			continue
		}
		start := index == 0 || morae[index-1].Pause
		end := index+1 == len(morae) || morae[index+1].Pause
		progress := float64(position) / float64(max(1, count-1))
		features := map[string]float64{
			"bias": 1, "position": progress, "position2": progress * progress,
			"tone_" + fmt.Sprint(tones[index]): 1,
		}
		if start {
			features["phrase_start"] = 1
		} else if tones[index-1] >= 1 && tones[index-1] <= 5 {
			features["prev_tone_"+fmt.Sprint(tones[index-1])] = 1
		}
		if end {
			features["phrase_end"] = 1
		} else if tones[index+1] >= 1 && tones[index+1] <= 5 {
			features["next_tone_"+fmt.Sprint(tones[index+1])] = 1
		}
		correction := model.MandarinCorrection(features)
		if len(correction) != len(model.Knots) {
			return curve
		}
		first := max(0, int(math.Ceil(timings[index].StartMS/curve.FrameMS)))
		last := min(len(result.Cents)-1, int(math.Floor((timings[index].StartMS+timings[index].DurationMS)/curve.FrameMS)))
		for frame := first; frame <= last; frame++ {
			unit := (float64(frame)*curve.FrameMS - timings[index].StartMS) / timings[index].DurationMS
			result.Cents[frame] += interpolateMandarinCorrection(model.Knots, correction, unit)
		}
		position++
	}
	return result
}

func interpolateMandarinCorrection(knots, values []float64, position float64) float64 {
	if position <= knots[0] {
		return values[0]
	}
	for index := 1; index < len(knots); index++ {
		if position <= knots[index] {
			ratio := (position - knots[index-1]) / (knots[index] - knots[index-1])
			return values[index-1]*(1-ratio) + values[index]*ratio
		}
	}
	return values[len(values)-1]
}

func (chineseProfile) ApplyBoundaryTone(_ Config, curve *render.PitchCurve, _ float64, _ bool) *render.PitchCurve {
	return curve
}

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
	phoneWeights := speechPhoneDurations(morae, plan.DefaultMoraDurationMS)
	for i, timing := range timings {
		if morae[i].Pause || timing.DurationMS <= 0 || tones[i] < 1 || tones[i] > 5 {
			continue
		}
		phraseFinal := i+1 == len(morae) || morae[i+1].Pause
		points := mandarinTonePoints(tones[i], phraseFinal)
		if tones[i] == 5 {
			previous := 0
			if i > 0 && !morae[i-1].Pause {
				previous = tones[i-1]
			}
			points = mandarinNeutralTonePoints(previous)
		}
		// 声調は頭子音を除く母音核と鼻音韻尾へ置く。
		startOffset, span := mandarinToneWindow(morae[i], phoneWeights[i], timing.DurationMS)
		first := max(0, int(math.Ceil(timing.StartMS/mandarinPitchFrameMS)))
		last := min(len(curve.Cents)-1, int(math.Floor((timing.StartMS+timing.DurationMS)/mandarinPitchFrameMS)))
		for frame := first; frame <= last; frame++ {
			position := (float64(frame)*mandarinPitchFrameMS - timing.StartMS - startOffset) / span
			curve.Cents[frame] = interpolateTone(points, max(0, min(1, position)))
		}
	}
	return curve
}

// 母音核が不明なら、語頭子音の合計長を使う。
func mandarinToneWindow(mora frontend.Mora, weights []float64, durationMS float64) (float64, float64) {
	spans := phoneSpansFromWeights(weights, durationMS)
	nucleusStart, nucleusEnd := -1, -1
	for j, p := range mora.Phones {
		// 鼻音韻尾も有声の韻に含め、声調が母音の途中で終わらないようにする。
		if p.Role == "nucleus" || p.Role == "coda" || p.Role == "medial" || p.Role == "offglide" {
			if nucleusStart < 0 {
				nucleusStart = j
			}
			nucleusEnd = j
		}
	}
	if nucleusStart < 0 {
		onset := 0.0
		for j, p := range mora.Phones {
			if p.Role == "onset" {
				onset += spans[j]
			}
		}
		return onset, math.Max(1, durationMS-onset)
	}
	start := 0.0
	for j := 0; j < nucleusStart; j++ {
		start += spans[j]
	}
	end := 0.0
	for j := 0; j <= nucleusEnd; j++ {
		end += spans[j]
	}
	return start, math.Max(1, end-start)
}

// 軽声は前の声調に合わせ、語尾へ少し下げる。
func mandarinNeutralTonePoints(previous int) []tonePoint {
	switch previous {
	case 1, 2, 3, 4:
	default:
		// 前が軽声または不明のときは中低で短く保つ。
		previous = 5
	}
	end := map[int]float64{1: -100, 2: -55, 3: 65, 4: -130, 5: -35}[previous]
	return []tonePoint{{0, end + 25}, {1, end}}
}

// 語彙上の声調を残したまま連続変調を求める。
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
			// 数列と年号ではyi1を保つ。
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
	tones := mandarinSurfaceTones(morae)
	for i, m := range morae {
		factor := 1.0
		energy := 1.0
		if tones[i] == 5 {
			// 軽声は短く弱く、前の声調の高さを保つ。
			factor = 0.62
			energy = 0.80
		} else if tones[i] == 4 {
			energy = 1.03
		}
		if !m.Pause && (i+1 == len(morae) || morae[i+1].Pause) {
			factor *= 1.12
			energy *= 0.97
		}
		result[i] = prosody.Prediction{DurationFactor: factor, PitchFactor: 1, EnergyFactor: energy}
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
