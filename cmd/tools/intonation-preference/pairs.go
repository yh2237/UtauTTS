package main

import (
	"context"
	"fmt"
	"math"
	"sort"

	"utautts/internal/plugin"
	"utautts/internal/render"
	"utautts/internal/tts"
)

const (
	minimumContourDifferenceCents = 25.0
	minimumDifferentFrames        = 20 // 10msフレームで少なくとも200ms
	maximumContourPairsPerPrompt  = 3
)

func buildPairs(m manifest, catalog *plugin.Catalog, bridge string) ([]pairOption, error) {
	if m.Mode != "contour" {
		var pairs []pairOption
		for _, item := range m.Prompts {
			for left := 0; left < len(m.Candidates); left++ {
				for right := left + 1; right < len(m.Candidates); right++ {
					pairs = append(pairs, pairOption{prompt: item, left: m.Candidates[left], right: m.Candidates[right]})
				}
			}
		}
		return pairs, nil
	}
	if len(m.Candidates) < 2 {
		return nil, fmt.Errorf("contour comparison needs a baseline and alternatives")
	}
	baseline := m.Candidates[0]
	var pairs []pairOption
	for _, item := range m.Prompts {
		cfg := tts.Config{Context: context.Background(), VoicebankPath: m.Voicebank, Text: item.Text,
			Tone: "C4", MoraDurationMS: m.MoraMS, PauseDurationMS: m.PauseMS,
			ApplyPitch: true, IntonationStrength: m.BaseStrength, ProsodyModelPath: m.ModelFile}
		if _, err := tts.ApplyRenderer(&cfg, catalog, m.Renderer, bridge); err != nil {
			return nil, err
		}
		preview, err := tts.PredictProsody(cfg)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", item.ID, err)
		}
		base, err := transformContour(preview, item.Text, baseline)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", item.ID, err)
		}
		active := activeFrames(preview, base)
		type impactful struct {
			profile candidate
			frames  int
		}
		var choices []impactful
		for _, profile := range m.Candidates[1:] {
			curve, err := transformContour(preview, item.Text, profile)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", item.ID, err)
			}
			frames := distinctPitchFrames(base, curve, active, minimumContourDifferenceCents)
			if frames >= minimumDifferentFrames {
				choices = append(choices, impactful{profile: profile, frames: frames})
			}
		}
		sort.SliceStable(choices, func(left, right int) bool { return choices[left].frames > choices[right].frames })
		for _, choice := range choices[:min(len(choices), maximumContourPairsPerPrompt)] {
			pairs = append(pairs, pairOption{prompt: item, left: baseline, right: choice.profile})
		}
	}
	if len(pairs) == 0 {
		return nil, fmt.Errorf("no contour candidates differ enough from the baseline; try other prompts or candidates")
	}
	return pairs, nil
}

func distinctPitchFrames(left, right *render.PitchCurve, active []bool, minimumCents float64) int {
	if left == nil || right == nil || left.FrameMS != right.FrameMS || len(left.Cents) != len(right.Cents) {
		return 0
	}
	count := 0
	for index := range left.Cents {
		if index < len(active) && active[index] && math.Abs(left.Cents[index]-right.Cents[index]) >= minimumCents {
			count++
		}
	}
	return count
}
