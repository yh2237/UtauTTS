package tts

import (
	"context"
	"fmt"
	"math"

	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/prosody"
	"github.com/yh2237/UtauTTS/internal/render"
	"github.com/yh2237/UtauTTS/internal/settings"
	"github.com/yh2237/UtauTTS/internal/speechtiming"
)

// unifiedF0Headは選んだ抑揚モデルが持つ統合韻律モデル（F0ヘッド）。持たなければnil。
func unifiedF0Head(model *prosody.Model) *speechtiming.TCN {
	if model == nil {
		return nil
	}
	return model.F0Head
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

// 自然スケールは強さ/既定値で拡大し、蒸留モデルは従来の拡大方式を使う。
func scaleUnifiedPitchCurve(contour *render.PitchCurve, model *speechtiming.TCN, strength float64) *render.PitchCurve {
	if model.F0Scale() > 0 {
		return scaleAutomaticPitchCurve(contour, strength)
	}
	if contour == nil || strength <= 0 {
		return nil
	}
	factor := strength / settings.Number("intonation_strength")
	result := &render.PitchCurve{FrameMS: contour.FrameMS, Cents: make([]float64, len(contour.Cents))}
	for index, cents := range contour.Cents {
		result.Cents[index] = cents * factor
	}
	return result
}

// weightはbの重み。bより後ろのフレームはaの値を保つ。
func blendPitchCurves(a, b *render.PitchCurve, weight float64) *render.PitchCurve {
	if a == nil || b == nil || a.FrameMS != b.FrameMS {
		return a
	}
	result := &render.PitchCurve{FrameMS: a.FrameMS, Cents: append([]float64(nil), a.Cents...)}
	for index := range result.Cents {
		if index < len(b.Cents) {
			result.Cents[index] = (1-weight)*a.Cents[index] + weight*b.Cents[index]
		}
	}
	return result
}

// unifiedProsodyPositionsは文内の位置の特徴（train-speech-timingのpositionFeaturesと同じ定義）をモーラごとに返す。
// 0 発話のモーラの通し位置、1 アクセント句の通し位置、2 最後のアクセント句、3 最後の息継ぎ区間、4 疑問文。休止（特徴が空）は疑問文だけを持つ。
func unifiedProsodyPositions(features []prosody.FeatureFrame, question bool) [][speechtiming.PositionFeatures]float32 {
	result := make([][speechtiming.PositionFeatures]float32, len(features))
	pause := func(index int) bool { return len(features[index]) == 0 }
	speech, phrases, lastGroup := 0, 0, 0
	for index := range features {
		if pause(index) {
			continue
		}
		if features[index]["accent_phrase_start"] != 0 || phrases == 0 {
			phrases++
		}
		if index > 0 && pause(index-1) {
			lastGroup = speech
		}
		speech++
	}
	position, phrase := 0, -1
	for index := range features {
		if question {
			result[index][4] = 1
		}
		if pause(index) {
			continue
		}
		if features[index]["accent_phrase_start"] != 0 || phrase < 0 {
			phrase++
		}
		result[index][0] = positionRatio(position, speech)
		result[index][1] = positionRatio(phrase, phrases)
		if phrase == phrases-1 {
			result[index][2] = 1
		}
		if position >= lastGroup {
			result[index][3] = 1
		}
		position++
	}
	return result
}

func positionRatio(index, count int) float32 {
	if count <= 1 {
		return 0
	}
	return float32(index) / float32(count-1)
}

func unifiedProsodyExtraVector(frame prosody.FeatureFrame, width int, pos, groups []string) []float32 {
	extra := make([]float32, width)
	accent := unifiedProsodyAccentVector(frame)
	copy(extra[:12], accent[:])
	posIndex := len(pos)
	for index, name := range pos {
		if frame["pos="+name] != 0 {
			posIndex = index
			break
		}
	}
	extra[12+posIndex] = 1
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
func unifiedProsodyContext(model *speechtiming.TCN, language string, features []prosody.FeatureFrame, timings []prosody.MoraTiming, durationMS float64, synthesisPlan *plan.Plan, question bool) ([][3]int, [][]float32, []bool, error) {
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
	var positions [][speechtiming.PositionFeatures]float32
	if model.F0Position() {
		positions = unifiedProsodyPositions(features, question)
	}
	pos, groups := model.PosVocab(), model.PosGroup1Vocab()
	for index, timing := range timings {
		var vector []float32
		if index < len(features) {
			vector = unifiedProsodyExtraVector(features[index], model.F0Context()-2, pos, groups)
			if positions != nil {
				copy(vector[len(vector)-speechtiming.PositionFeatures:], positions[index][:])
			}
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

type unifiedProsodyPrediction struct {
	contour *render.PitchCurve
	energy  []float32
	ids     [][3]int
}

func predictUnifiedProsody(ctx context.Context, model *speechtiming.TCN, language string, features []prosody.FeatureFrame, timings []prosody.MoraTiming, durationMS float64, synthesisPlan *plan.Plan, question, includeEnergy bool) unifiedProsodyPrediction {
	if model == nil || !model.HasF0Head() {
		return unifiedProsodyPrediction{}
	}
	ids, cont, speech, err := unifiedProsodyContext(model, language, features, timings, durationMS, synthesisPlan, question)
	if err != nil {
		return unifiedProsodyPrediction{}
	}
	cache, err := cacheFor(ctx)
	if err != nil {
		return unifiedProsodyPrediction{}
	}
	key := unifiedProsodyKey(model, ids, cont, speech)
	cache.RLock()
	cached, ok := cache.unified[key]
	cache.RUnlock()
	if ok {
		return cached.copy(includeEnergy)
	}
	values, energy, err := model.PredictProsody(ids, cont)
	if err != nil || len(values) == 0 {
		return unifiedProsodyPrediction{}
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
		cents = prosody.SmoothPitchPhrases(cents, speech, 10, 20)
		prosody.ClipPitchContour(cents, speech, 75, 90)
	}
	prediction := unifiedProsodyPrediction{contour: &render.PitchCurve{FrameMS: 10, Cents: cents}, energy: energy, ids: ids}
	cache.storeUnified(key, prediction)
	return prediction.copy(includeEnergy)
}

// previewPlanは音源を読まないプレビュー用に、モーラの時刻からF0ヘッドの入力に足りるプランを組む。
// 合成でも曲線を作る時点の先行発声と余白は0なので、休止と促音（無音の単位）を除くだけで同じ入力になる。
func previewPlan(morae []frontend.Mora, timings []prosody.MoraTiming, durationMS float64) *plan.Plan {
	result := &plan.Plan{DurationMS: durationMS}
	for index, mora := range morae {
		if index >= len(timings) || mora.Pause || mora.Text == "っ" || mora.Text == "ッ" {
			continue
		}
		result.Units = append(result.Units, plan.Unit{
			Position: index, Role: "mora", Mora: mora.Text,
			NoteStartMS: timings[index].StartMS, DurationMS: timings[index].DurationMS,
		})
	}
	return result
}

func (prediction unifiedProsodyPrediction) applyEnergy(model *speechtiming.TCN, synthesisPlan *plan.Plan) {
	values, ids := prediction.energy, prediction.ids
	if len(values) == 0 {
		return
	}
	// 学習目標に合わせ、休止を除いた平均で中心化する。
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
