package main

import (
	"errors"
	"math"
	"strings"

	"utautts/internal/render"
	"utautts/internal/tts"
)

// makeContourCandidatesは意図的に差をつけた範囲内の輪郭候補を返す。候補パラメータは再現性のためセッションマニフェストへ保存する。
func makeContourCandidates() []candidate {
	return []candidate{
		{ID: "base-1_0", Strength: 1},
		{ID: "base-1_4", Strength: 1.4},
		{ID: "detail", Strength: 1.15, Contrast: .6, OffsetCents: -10},
		{ID: "declination", Strength: 1.1, OffsetCents: -10, DeclinationCents: -20, StatementTailCents: -14, QuestionTailCents: 14},
		{ID: "detail-declination", Strength: 1.15, Contrast: .6, OffsetCents: -10, DeclinationCents: -18, StatementTailCents: -16, QuestionTailCents: 18},
		{ID: "energetic", Strength: 1.25, Contrast: .9, OffsetCents: -12, DeclinationCents: -12, StatementTailCents: -18, QuestionTailCents: 20},
		{ID: "restrained", Strength: 1, Contrast: .3, OffsetCents: -10, DeclinationCents: -14, StatementTailCents: -12, QuestionTailCents: 12},
		{ID: "wide-ending", Strength: 1.2, Contrast: .45, OffsetCents: -10, DeclinationCents: -22, StatementTailCents: -24, QuestionTailCents: 28},
	}
}

func formatCandidateIDs(candidates []candidate) string {
	values := make([]string, len(candidates))
	for index, item := range candidates {
		values[index] = item.ID
	}
	return strings.Join(values, ", ")
}

// transformContourは有声モーラ内だけモデル輪郭を変更する。ポーズは0のままにして、句内調整が無音をまたいで伝播しないようにする。
func transformContour(preview *tts.ProsodyPreview, text string, profile candidate) (*render.PitchCurve, error) {
	if preview == nil || preview.FramePitchCurve == nil || preview.FramePitchCurve.FrameMS <= 0 {
		return nil, errors.New("prosody preview did not contain a frame pitch curve")
	}
	base := preview.FramePitchCurve
	result := &render.PitchCurve{FrameMS: base.FrameMS, Cents: make([]float64, len(base.Cents))}
	active := activeFrames(preview, base)
	question := strings.HasSuffix(strings.TrimSpace(text), "?") || strings.HasSuffix(strings.TrimSpace(text), "？")
	for start := 0; start < len(base.Cents); {
		for start < len(base.Cents) && !active[start] {
			start++
		}
		end := start
		for end < len(base.Cents) && active[end] {
			end++
		}
		if start < end {
			transformSpan(result.Cents[start:end], base.Cents[start:end], profile, question)
		}
		start = end + 1
	}
	return result, nil
}

func activeFrames(preview *tts.ProsodyPreview, curve *render.PitchCurve) []bool {
	active := make([]bool, len(curve.Cents))
	for index, mora := range preview.Morae {
		if mora.Pause || index >= len(preview.MoraDurationsMS) || index >= len(preview.MoraPositionsMS) {
			continue
		}
		start := preview.MoraPositionsMS[index] - preview.MoraDurationsMS[index]/2
		end := preview.MoraPositionsMS[index] + preview.MoraDurationsMS[index]/2
		for frame := range active {
			position := float64(frame) * curve.FrameMS
			if position >= start && position < end {
				active[frame] = true
			}
		}
	}
	return active
}

func transformSpan(output, input []float64, profile candidate, question bool) {
	const localRadius = 8
	if len(output) == 0 {
		return
	}
	scaled := make([]float64, len(input))
	for index, value := range input {
		scaled[index] = value * profile.Strength
	}
	tail := profile.StatementTailCents
	if question {
		tail = profile.QuestionTailCents
	}
	tailLength := int(math.Ceil(float64(len(output)) * .3))
	if tailLength < 1 {
		tailLength = 1
	}
	tailStart := len(output) - tailLength
	for index, value := range scaled {
		left := max(0, index-localRadius)
		right := min(len(scaled), index+localRadius+1)
		mean := 0.0
		for _, local := range scaled[left:right] {
			mean += local
		}
		mean /= float64(right - left)
		progress := (float64(index) + .5) / float64(len(output))
		adjusted := mean + (value-mean)*(1+profile.Contrast)
		adjusted += profile.OffsetCents + profile.DeclinationCents*(progress-.5)
		if index >= tailStart {
			t := float64(index-tailStart+1) / float64(tailLength)
			adjusted += tail * t * t * (3 - 2*t)
		}
		output[index] = adjusted
	}
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func max(left, right int) int {
	if left > right {
		return left
	}
	return right
}
