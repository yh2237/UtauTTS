package worldrender

import (
	"fmt"

	"utautts/internal/provider"
	"utautts/internal/speechtiming"
)

type timingWarp struct {
	Strength        float64
	LeadingMarginMS float64
	Morae           []speechtiming.Mora
}

func timingWarpFromJob(job *provider.TimingWarp) *timingWarp {
	if job == nil || job.Strength <= 0 {
		return nil
	}
	result := &timingWarp{Strength: job.Strength, LeadingMarginMS: job.LeadingMarginMS}
	for _, mora := range job.Morae {
		result.Morae = append(result.Morae, speechtiming.Mora{
			Text: mora.Text, NoteStartMS: mora.NoteStartMS, DurationMS: mora.DurationMS,
			EffectivePreutteranceMS: mora.ConsonantMS,
		})
	}
	return result
}

func applyTimingWarp(warp *timingWarp, sampleRate int, features *worldFeatures) error {
	if warp == nil || warp.Strength <= 0 || len(warp.Morae) == 0 {
		return nil
	}
	model, err := speechtiming.DefaultTarget()
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
