package worldrender

import (
	"fmt"

	"utautts/internal/provider"
	"utautts/internal/speechtiming"
)

type timingWarp struct {
	Strength float64
	Language string
	Timeline speechtiming.Timeline
}

func timingWarpFromJob(job *provider.TimingWarp) *timingWarp {
	if job == nil || job.Strength <= 0 {
		return nil
	}
	timeline := speechtiming.Timeline{Starts: job.Starts, Ends: job.Ends}
	for _, span := range job.Spans {
		timeline.Spans = append(timeline.Spans, speechtiming.Span{Label: span.Label, Start: span.Start, End: span.End})
	}
	return &timingWarp{Strength: job.Strength, Language: job.Language, Timeline: timeline}
}

func applyTimingWarp(warp *timingWarp, sampleRate int, features *worldFeatures) error {
	if warp == nil || warp.Strength <= 0 || len(warp.Timeline.Spans) == 0 {
		return nil
	}
	model, err := speechtiming.TargetForLanguage(warp.Language)
	if err != nil {
		return fmt.Errorf("load timing warp model: %w", err)
	}
	warped, err := speechtiming.Warp(model, warp.Timeline, speechtiming.Features{
		Frames: features.Frames, FFTSize: features.FFTSize, SampleRate: sampleRate,
		F0: features.F0, Spectrum: features.Spectrum, Aperiodicity: features.Aperiodicity,
	}, warp.Strength)
	if err != nil {
		return fmt.Errorf("timing warp: %w", err)
	}
	features.F0, features.Spectrum, features.Aperiodicity = warped.F0, warped.Spectrum, warped.Aperiodicity
	return nil
}
