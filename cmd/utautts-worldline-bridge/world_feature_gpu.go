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
	if frames < 2 || fftSize < 2 || fftSize%2 != 0 || len(prepared) == 0 || len(input.Units) != len(prepared) {
		return worldFeatures{}, fmt.Errorf("invalid CUDA WORLD feature dimensions")
	}
	bins := fftSize/2 + 1
	units := make([]cudaWorldUnit, len(prepared))
	totalSourceFrames := 0
	// Cached features are immutable. Repeated units share their backing arrays;
	// transfer each distinct analysis once, retaining independent unit timing.
	offsets := make(map[[3]*float64]int)
	unique := make([]int, 0, len(prepared))
	for index, entry := range prepared {
		item := input.Units[index]
		features := entry.cached.features
		if features.Frames < 2 || features.FFTSize != fftSize || len(features.F0) != features.Frames || len(features.Spectrum) != features.Frames*bins || len(features.Aperiodicity) != features.Frames*bins {
			return worldFeatures{}, fmt.Errorf("invalid CUDA WORLD source dimensions at unit %d", index)
		}
		key := [3]*float64{&features.F0[0], &features.Spectrum[0], &features.Aperiodicity[0]}
		offset, found := offsets[key]
		if !found {
			offset = totalSourceFrames
			offsets[key] = offset
			unique = append(unique, index)
			totalSourceFrames += features.Frames
			if totalSourceFrames > 2147483647 {
				return worldFeatures{}, fmt.Errorf("CUDA WORLD source exceeds ABI frame limit")
			}
		}
		units[index] = cudaWorldUnit{
			FeatureOffset: int32(offset), FeatureFrames: int32(features.Frames),
			DurationMS: entry.cached.duration, PositionMS: item.PositionMS,
			LengthMS: item.LengthMS, FadeInMS: item.FadeInMS, FadeOutMS: item.FadeOutMS,
			SkipMS: item.SkipMS, ConsonantMS: item.ConsonantMS, RequiredMS: item.RequiredLengthMS,
			ConsonantVelocity: item.ConsonantVelocity, Volume: item.Volume,
		}
	}
	sourceF0 := make([]float64, totalSourceFrames)
	sourceSpectrum := make([]float64, totalSourceFrames*bins)
	sourceAP := make([]float64, totalSourceFrames*bins)
	for _, index := range unique {
		entry := prepared[index]
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
