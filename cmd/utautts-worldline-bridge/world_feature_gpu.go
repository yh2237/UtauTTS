package main

import "fmt"

type cudaWorldUnit struct {
	FeatureOffset, FeatureFrames int32
	DurationMS, PositionMS       float64
	LengthMS, FadeInMS           float64
	FadeOutMS, SkipMS            float64
	ConsonantMS, RequiredMS      float64
	ConsonantVelocity, Volume    float64
}

func mixWorldFeaturesCUDA(path string, input manifest, prepared []preparedWorldUnit, fftSize int) (worldFeatures, error) {
	frames := len(input.F0Curve)
	bins := fftSize/2 + 1
	units := make([]cudaWorldUnit, len(prepared))
	totalSourceFrames := 0
	for index, entry := range prepared {
		item := input.Units[index]
		features := entry.cached.features
		units[index] = cudaWorldUnit{
			FeatureOffset: int32(totalSourceFrames), FeatureFrames: int32(features.Frames),
			DurationMS: entry.cached.duration, PositionMS: item.PositionMS,
			LengthMS: item.LengthMS, FadeInMS: item.FadeInMS, FadeOutMS: item.FadeOutMS,
			SkipMS: item.SkipMS, ConsonantMS: item.ConsonantMS, RequiredMS: item.RequiredLengthMS,
			ConsonantVelocity: item.ConsonantVelocity, Volume: item.Volume,
		}
		totalSourceFrames += features.Frames
	}
	sourceF0 := make([]float64, totalSourceFrames)
	sourceSpectrum := make([]float64, totalSourceFrames*bins)
	sourceAP := make([]float64, totalSourceFrames*bins)
	for index, entry := range prepared {
		offset := int(units[index].FeatureOffset)
		features := entry.cached.features
		copy(sourceF0[offset:], features.F0)
		copy(sourceSpectrum[offset*bins:], features.Spectrum)
		copy(sourceAP[offset*bins:], features.Aperiodicity)
	}
	result := worldFeatures{
		Frames: frames, FFTSize: fftSize, F0: make([]float64, frames),
		Spectrum: make([]float64, frames*bins), Aperiodicity: make([]float64, frames*bins),
	}
	if err := invokeCUDAWorldFeatureMix(path, input.F0Curve, fftSize, units,
		sourceF0, sourceSpectrum, sourceAP, result.F0, result.Spectrum, result.Aperiodicity); err != nil {
		return worldFeatures{}, fmt.Errorf("CUDA WORLD feature mix: %w", err)
	}
	return result, nil
}
