package main

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"utautts/internal/atomicfile"
	"utautts/internal/frontend"
	"utautts/internal/pitch"
	"utautts/internal/synth"
)

func validateProsodyExperiment(mode, renderer, model, modelFile string, diagnose bool, prompts []prompt) error {
	switch mode {
	case "baseline":
		return nil
	case "timing", "pitch", "both":
	default:
		return fmt.Errorf("unknown prosody experiment %q", mode)
	}
	if renderer != "utautts-world-phrase" || model != "none" || modelFile != "" || diagnose {
		return fmt.Errorf("prosody experiment requires --renderers utautts-world-phrase --model none and synthesis")
	}
	for _, p := range prompts {
		_, ph, err := frontend.ResolveLanguage(p.Language, p.Phonemizer)
		if err != nil {
			return err
		}
		if ph != frontend.PhonemizerEnglishDelta && ph != frontend.PhonemizerEnglishVCCV && ph != frontend.PhonemizerChinese {
			return fmt.Errorf("case %s: experiment requires en-delta, en-vccv or zh-cvvc", p.ID)
		}
	}
	return nil
}

type pitchFrame struct {
	AudioMS    float64  `json:"audio_ms"`
	PlanMS     float64  `json:"plan_ms"`
	TargetHz   float64  `json:"target_hz"`
	MeasuredHz float64  `json:"measured_hz"`
	ErrorCents *float64 `json:"error_cents,omitempty"`
}

type pitchTrace struct {
	Estimator           string       `json:"estimator"`
	FrameMS             float64      `json:"frame_ms"`
	WindowMS            float64      `json:"window_ms"`
	PairedFrames        int          `json:"paired_frames"`
	MedianAbsoluteCents float64      `json:"median_absolute_cents"`
	P90AbsoluteCents    float64      `json:"p90_absolute_cents"`
	Frames              []pitchFrame `json:"frames"`
}

func writePitchTrace(path string, result *synth.Result) error {
	if result.RenderReport == nil || result.RenderReport.TargetF0 == nil {
		return fmt.Errorf("renderer did not report target F0")
	}
	track := result.RenderReport.TargetF0
	trace := pitchTrace{Estimator: "internal/pitch.Estimate; 60-500 Hz; zero means unmeasured/unvoiced; target is before WORLD voicing", FrameMS: track.FrameMS, WindowMS: 40}
	wave := make([]float64, len(result.Audio.Data))
	for i, s := range result.Audio.Data {
		wave[i] = float64(s) / 32768
	}
	rate := result.Audio.SampleRate
	window := int(math.Round(float64(rate) * .04))
	var errors []float64
	for i, target := range track.Hz {
		audioMS := float64(i) * track.FrameMS
		frame := pitchFrame{AudioMS: audioMS, PlanMS: track.StartMS + audioMS, TargetHz: target}
		center := int(math.Round(audioMS * float64(rate) / 1000))
		left := center - window/2
		if left >= 0 && left+window <= len(wave) {
			frame.MeasuredHz = pitch.Estimate(wave[left:left+window], rate)
		}
		if target >= 60 && target <= 500 && frame.MeasuredHz > 0 {
			delta := 1200 * math.Log2(frame.MeasuredHz/target)
			frame.ErrorCents = &delta
			errors = append(errors, math.Abs(delta))
		}
		trace.Frames = append(trace.Frames, frame)
	}
	sort.Float64s(errors)
	trace.PairedFrames = len(errors)
	if len(errors) > 0 {
		trace.MedianAbsoluteCents = errors[len(errors)/2]
		trace.P90AbsoluteCents = errors[min(len(errors)-1, int(math.Ceil(float64(len(errors))*.9))-1)]
	}
	data, err := json.MarshalIndent(trace, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.WriteFile(path, data)
}
