package worldrender

import (
	"fmt"

	"utautts/internal/provider"
	"utautts/internal/speechtiming"
)

type timingWarp struct {
	Strength        float64
	LeadingMarginMS float64
	Language        string
	Morae           []speechtiming.Mora
}

func timingWarpFromJob(job *provider.TimingWarp) *timingWarp {
	if job == nil || job.Strength <= 0 {
		return nil
	}
	result := &timingWarp{Strength: job.Strength, LeadingMarginMS: job.LeadingMarginMS, Language: job.Language}
	for _, mora := range job.Morae {
		entry := speechtiming.Mora{
			Text: mora.Text, NoteStartMS: mora.NoteStartMS, DurationMS: mora.DurationMS,
			EffectivePreutteranceMS: mora.ConsonantMS,
		}
		for _, span := range mora.Spans {
			entry.Spans = append(entry.Spans, speechtiming.PhoneSpan{
				Label: span.Symbol, StartMS: span.StartMS, DurationMS: span.DurationMS,
			})
		}
		result.Morae = append(result.Morae, entry)
	}
	return result
}

func applyTimingWarp(warp *timingWarp, sampleRate int, features *worldFeatures) error {
	if warp == nil || warp.Strength <= 0 || len(warp.Morae) == 0 {
		return nil
	}
	model, err := speechtiming.TargetForLanguage(warp.Language)
	if err != nil {
		return fmt.Errorf("load timing warp model: %w", err)
	}
	warped, err := speechtiming.Warp(model, warp.Morae, warp.LeadingMarginMS, speechtiming.Features{
		Frames: features.Frames, FFTSize: features.FFTSize, SampleRate: sampleRate,
		F0: features.F0, Spectrum: features.Spectrum, Aperiodicity: features.Aperiodicity,
	}, warp.Strength)
	if err != nil {
		return fmt.Errorf("timing warp: %w", err)
	}
	features.F0, features.Spectrum, features.Aperiodicity = warped.F0, warped.Spectrum, warped.Aperiodicity
	return nil
}
