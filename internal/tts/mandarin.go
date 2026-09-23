package tts

import (
	"fmt"
	"math"
	"strings"

	"utautts/internal/frontend"
	"utautts/internal/prosody"
	"utautts/internal/render"
)

// chineseProfileは中国語のPinyin解析と声調曲線をまとめる。
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

func (chineseProfile) ProsodyModelFallback(string) string { return "" }

func (chineseProfile) SupportsStretchAdapt() bool { return false }

func (chineseProfile) PhoneTiming(_ Config, morae []frontend.Mora, _ bool) ([][]float64, string) {
	return languagePhoneWeights(frontend.LanguageChinese, morae), "language-phone-v1"
}

func (chineseProfile) Predict(morae []frontend.Mora) []prosody.Prediction {
	return mandarinPredictions(morae)
}

func (chineseProfile) AdjustPredictions(_ Config, _ *prosody.Model, _ []frontend.Mora, predictions []prosody.Prediction, _ []prosody.FeatureFrame) []prosody.Prediction {
	return predictions
}

func (chineseProfile) AutomaticPitchCurve(_ Config, _ *prosody.Model, morae []frontend.Mora, timings []prosody.MoraTiming, durationMS float64) (*render.PitchCurve, bool) {
	curve := mandarinToneCurve(morae, timings, durationMS)
	return curve, curve != nil
}

func (chineseProfile) ApplyBoundaryTone(_ Config, curve *render.PitchCurve, _ float64, _ bool) *render.PitchCurve {
	return curve
}

func (chineseProfile) ExperimentalPitchAllowed() bool { return true }

const mandarinPitchFrameMS = 10

type tonePoint struct {
	position float64
	cents    float64
}

// mandarinToneCurveは各音節の声調を母音核の区間へ置く。
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
	phoneWeights := languagePhoneWeights(frontend.LanguageChinese, morae)
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
		// 声調は無声の頭子音を除く母音核へ置く。
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

// mandarinToneWindowは声調を置く母音核区間の開始位置と長さを返す。
// 母音核が不明なときは頭子音の合計長でずらす従来動作へ戻す。
func mandarinToneWindow(mora frontend.Mora, weights []float64, durationMS float64) (float64, float64) {
	spans := phoneSpansFromWeights(weights, durationMS)
	nucleusStart, nucleusEnd := -1, -1
	for j, p := range mora.Phones {
		if p.Role == "nucleus" {
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

// mandarinNeutralTonePointsは軽声(5)のF0を前の声調から求める。
// 前声調が高いほど軽声も高く、いずれも語尾へ向けてわずかに下降する。
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
