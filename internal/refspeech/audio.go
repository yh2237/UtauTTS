package refspeech

import (
	"errors"
	"math"
	"time"

	"github.com/yh2237/UtauIDX/index"
	"github.com/yh2237/UtauIDX/vocoder"
)

type AudioOptions struct {
	ContinuityBonus *float64
	CenterHz        float64
	PitchScale      float64
	PitchSemitones  *float64
	Logf            func(string)
}

func (e *Engine) SpeakAudio(idx *index.Index, world index.World, audio []float64, sampleRate int, options AudioOptions) (Speech, error) {
	if err := validateSynthesis(idx, world, options.ContinuityBonus); err != nil {
		return Speech{}, err
	}
	if len(audio) == 0 || sampleRate <= 0 {
		return Speech{}, errors.New("reference audio and a positive sample rate are required")
	}
	if semitones := options.PitchSemitones; semitones != nil && (math.IsNaN(*semitones) || math.IsInf(*semitones, 0)) {
		return Speech{}, errors.New("pitch semitones must be finite")
	}
	started := time.Now()
	channels, _, frames, err := e.queryFeatures(idx, audio, sampleRate, "")
	if err != nil {
		return Speech{}, err
	}
	analysis, err := vocoder.AnalyzeReference(world, audio, sampleRate, frames, nil, 0)
	if err != nil {
		return Speech{}, err
	}
	vocoder.FillVoicedByLevel(analysis.F0, analysis.Levels, -35)
	var f0 []float64
	if semitones := options.PitchSemitones; semitones != nil {
		ratio := max(0.5, min(2.0, math.Exp2(*semitones/12)))
		f0 = make([]float64, len(analysis.F0))
		for i, hz := range analysis.F0 {
			f0[i] = hz * ratio
		}
	} else {
		f0 = outputF0(idx, analysis.F0, options.CenterHz, options.PitchScale)
	}
	output, err := synthesizeQuery(idx, world, channels, analysis, f0, options.ContinuityBonus, options.Logf)
	if err != nil {
		return Speech{}, err
	}
	if options.Logf != nil {
		options.Logf("合成: 全体 " + time.Since(started).Round(time.Millisecond).String())
	}
	return Speech{Audio: output, SampleRate: idx.SampleRate, F0: f0, AutomaticF0: f0}, nil
}
