package tts

// 統合韻律モデル（メル＋F0＋エネルギー）の適用。
// F0ヘッドの輪郭を自動ピッチ曲線に、エネルギーヘッドの値をプランのEnergyFactorに使う。
// モデルはinternal/speechtimingへ埋め込む。

import (
	"fmt"
	"math"
	"sort"

	"utautts/internal/frontend"
	"utautts/internal/plan"
	"utautts/internal/prosody"
	"utautts/internal/render"
	"utautts/internal/speechtiming"
)

// unifiedProsodyEnabledは統合韻律モデルの使用有無。nilは既定で無効。
func unifiedProsodyEnabled(cfg Config) bool {
	return cfg.UnifiedProsody != nil && *cfg.UnifiedProsody
}

func unifiedProsodyAccentVector(frame prosody.FeatureFrame) [12]float32 {
	return [12]float32{
		float32(frame["accent_position"]),
		float32(frame["accent_from_end"]),
		float32(frame["accent_nucleus_position"]),
		float32(frame["accent_high"]),
		float32(frame["accent_phrase_start"]),
		float32(frame["accent_phrase_end"]),
		float32(frame["word_start"]),
		float32(frame["word_end"]),
		float32(frame["accent_type=heiban"]),
		float32(frame["accent_type=before"]),
		float32(frame["accent_type=nucleus"]),
		float32(frame["accent_type=after"]),
	}
}

// unifiedProsodyExtraVectorはアクセント12＋POS one-hot＋pos_group1 one-hotを組む。
func unifiedProsodyExtraVector(frame prosody.FeatureFrame, model *speechtiming.TCN) []float32 {
	extra := make([]float32, model.F0Context()-2)
	accent := unifiedProsodyAccentVector(frame)
	copy(extra[:12], accent[:])
	pos := model.PosVocab()
	posIndex := len(pos)
	for index, name := range pos {
		if frame["pos="+name] != 0 {
			posIndex = index
			break
		}
	}
	extra[12+posIndex] = 1
	groups := model.PosGroup1Vocab()
	groupIndex := len(groups)
	for index, name := range groups {
		if frame["pos_group1="+name] != 0 {
			groupIndex = index
			break
		}
	}
	extra[12+len(pos)+1+groupIndex] = 1
	return extra
}

// unifiedProsodyContextはjaのプランからタイムラインとアクセント特徴を組み、文脈トランクの入力と発話マスクを返す。
func unifiedProsodyContext(model *speechtiming.TCN, language string, features []prosody.FeatureFrame, timings []prosody.MoraTiming, durationMS float64, synthesisPlan *plan.Plan) ([][3]int, [][]float32, []bool, error) {
	if model == nil || synthesisPlan == nil || frontend.NormalizeLanguage(language) != "ja" {
		return nil, nil, nil, fmt.Errorf("unified prosody unavailable")
	}
	transition := map[int]float64{}
	for _, unit := range synthesisPlan.Units {
		if unit.Role == "transition" && !unit.Silent {
			transition[unit.Position] = unit.DurationMS
		}
	}
	var morae []speechtiming.Mora
	for _, unit := range synthesisPlan.Units {
		if unit.Role != "mora" || unit.Silent || unit.Mora == "" {
			continue
		}
		morae = append(morae, speechtiming.Mora{
			Text: unit.Mora, NoteStartMS: unit.NoteStartMS, DurationMS: unit.DurationMS,
			EffectivePreutteranceMS: math.Max(unit.EffectivePreutteranceMS, transition[unit.Position]),
		})
	}
	if len(morae) == 0 {
		return nil, nil, nil, fmt.Errorf("unified prosody unavailable")
	}
	frames := int(math.Round(durationMS / 10))
	if frames < 2 {
		return nil, nil, nil, fmt.Errorf("unified prosody unavailable")
	}
	timeline := speechtiming.PhoneTimeline(morae, synthesisPlan.LeadingMarginMS, frames)
	extras := make([][]float32, frames)
	for index, timing := range timings {
		var vector []float32
		if index < len(features) {
			vector = unifiedProsodyExtraVector(features[index], model)
		}
		a := int(math.Round(timing.StartMS / 10))
		b := int(math.Round((timing.StartMS + timing.DurationMS) / 10))
		a, b = max(0, a), min(frames, max(b, a+1))
		for t := a; t < b; t++ {
			if vector != nil {
				extras[t] = vector
			}
		}
	}
	speech := make([]bool, frames)
	for _, span := range timeline.Spans {
		if span.Label == "sil" {
			continue
		}
		a := int(math.Round(span.Start * 1000 / 10))
		b := int(math.Round(span.End * 1000 / 10))
		a, b = max(0, a), min(frames, max(b, a+1))
		for t := a; t < b; t++ {
			speech[t] = true
		}
	}
	ids, cont, err := model.F0Inputs(timeline.Spans, extras, frames)
	if err != nil {
		return nil, nil, nil, err
	}
	return ids, cont, speech, nil
}

// unifiedProsodyContourはF0ヘッドの輪郭を自動ピッチ曲線として返す。
func unifiedProsodyContour(language string, features []prosody.FeatureFrame, timings []prosody.MoraTiming, durationMS float64, synthesisPlan *plan.Plan) *render.PitchCurve {
	model, err := speechtiming.ProsodyTarget()
	if err != nil || !model.HasF0Head() {
		return nil
	}
	ids, cont, speech, err := unifiedProsodyContext(model, language, features, timings, durationMS, synthesisPlan)
	if err != nil {
		return nil
	}
	values, err := model.PredictF0(ids, cont)
	if err != nil || len(values) == 0 {
		return nil
	}
	scale := model.F0Scale()
	if scale <= 0 {
		scale = 0.3 * 1200 / math.Ln2
	}
	cents := make([]float64, len(values))
	for t, value := range values {
		cents[t] = float64(value) * scale
	}
	if model.F0Scale() > 0 {
		// 蒸留モデルはv10と同じ後処理（平滑化20ms・p99 75cent・最大90cent）を再現する。
		cents = smoothPhraseContour(cents, speech, 2.0)
		cents = clipContourPercentile(cents, speech, 75, 90)
	}
	return &render.PitchCurve{FrameMS: 10, Cents: cents}
}

// smoothPhraseContourは発話区間ごとにGaussian平滑化する（v10のランタイムと同じ）。
func smoothPhraseContour(values []float64, speech []bool, sigma float64) []float64 {
	result := append([]float64(nil), values...)
	if sigma <= 0 || len(values) != len(speech) {
		return result
	}
	radius := max(1, int(math.Ceil(3*sigma)))
	weights := make([]float64, 2*radius+1)
	for offset := -radius; offset <= radius; offset++ {
		weights[offset+radius] = math.Exp(-0.5 * math.Pow(float64(offset)/sigma, 2))
	}
	for start := 0; start < len(values); {
		if !speech[start] {
			start++
			continue
		}
		end := start + 1
		for end < len(values) && speech[end] {
			end++
		}
		for position := start; position < end; position++ {
			sum, weightSum := 0.0, 0.0
			for offset := -radius; offset <= radius; offset++ {
				source := max(start, min(end-1, position+offset))
				weight := weights[offset+radius]
				sum += values[source] * weight
				weightSum += weight
			}
			result[position] = sum / weightSum
		}
		start = end
	}
	return result
}

// clipContourPercentileは発話区間のp99を上限へ揃え、最大値でクリップする（v10のランタイムと同じ）。
func clipContourPercentile(values []float64, speech []bool, p99, maximum float64) []float64 {
	var selected []float64
	for index, value := range values {
		if index < len(speech) && speech[index] {
			selected = append(selected, math.Abs(value))
		}
	}
	if len(selected) > 0 {
		sort.Float64s(selected)
		index := int(math.Ceil(0.99*float64(len(selected)))) - 1
		observed := selected[max(0, min(len(selected)-1, index))]
		if observed > p99 && observed > 0 {
			gain := p99 / observed
			for i := range values {
				if i < len(speech) && speech[i] {
					values[i] *= gain
				}
			}
		}
	}
	for i := range values {
		values[i] = math.Min(maximum, math.Max(-maximum, values[i]))
	}
	return values
}

// applyUnifiedProsodyEnergyはエネルギーヘッドの値を平滑化し、プランのEnergyFactorへ適用する。
func applyUnifiedProsodyEnergy(language string, features []prosody.FeatureFrame, timings []prosody.MoraTiming, durationMS float64, synthesisPlan *plan.Plan) {
	model, err := speechtiming.ProsodyTarget()
	if err != nil || !model.HasEnergyHead() {
		return
	}
	ids, cont, _, err := unifiedProsodyContext(model, language, features, timings, durationMS, synthesisPlan)
	if err != nil {
		return
	}
	values, err := model.PredictEnergy(ids, cont)
	if err != nil || len(values) == 0 {
		return
	}
	// 学習目標は発話内で中心化されているため、推論値も有声フレームの平均で中心化する。
	silence := -1
	for index, name := range model.Phones() {
		if name == "sil" {
			silence = index
			break
		}
	}
	sum, count := 0.0, 0
	for t, value := range values {
		if silence >= 0 && t < len(ids) && ids[t][0] == silence {
			continue
		}
		sum += float64(value)
		count++
	}
	if count > 0 {
		center := float32(sum / float64(count))
		for t := range values {
			values[t] -= center
		}
	}
	smoothed := movingAverage(values, 5)
	for index := range synthesisPlan.Units {
		unit := &synthesisPlan.Units[index]
		if unit.Role != "mora" || unit.Silent || unit.Mora == "" {
			continue
		}
		a := int(math.Round(unit.NoteStartMS / 10))
		b := int(math.Round((unit.NoteStartMS + unit.DurationMS) / 10))
		a, b = max(0, a), min(len(smoothed), max(b, a+1))
		if a >= len(smoothed) {
			continue
		}
		mean := 0.0
		for _, value := range smoothed[a:b] {
			mean += float64(value)
		}
		mean /= float64(b - a)
		gain := math.Pow(10, mean/2)
		unit.EnergyFactor = math.Min(1.3, math.Max(0.75, gain))
	}
}

func movingAverage(values []float32, window int) []float32 {
	if window <= 1 || len(values) == 0 {
		return values
	}
	out := make([]float32, len(values))
	half := window / 2
	for i := range values {
		sum, count := 0.0, 0
		for j := max(0, i-half); j <= min(len(values)-1, i+half); j++ {
			sum += float64(values[j])
			count++
		}
		out[i] = float32(sum / float64(count))
	}
	return out
}
