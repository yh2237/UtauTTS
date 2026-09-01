package main

import (
	"fmt"
	"math"
)

type cachedWorldUnit struct {
	features worldFeatures
	duration float64
}

type preparedWorldUnit struct {
	cached cachedWorldUnit
}

func renderUtauTTSWorldPhrase(engine worldEngine, input manifest, cache *worldFeatureCache) ([]float32, error) {
	frames := len(input.F0Curve)
	if frames < 2 {
		return nil, fmt.Errorf("WORLD phrase has no frames")
	}
	var fftSize int
	prepared := make([]preparedWorldUnit, len(input.Units))
	for index, item := range input.Units {
		key := item.CacheKey
		if key == "" {
			key = fmt.Sprintf("%s|%.4f|%.4f", item.Source, item.OffsetMS, item.CutoffMS)
		}
		entry, found := cache.get(key)
		if !found {
			sampleRate, samples, err := readPCM16(item.Source)
			if err != nil {
				return nil, fmt.Errorf("read WORLD unit %q: %w", item.Source, err)
			}
			if sampleRate != input.SampleRate {
				return nil, fmt.Errorf("WORLD unit sample rate is %d, expected %d", sampleRate, input.SampleRate)
			}
			features, duration, err := analyzeWorldUnit(engine, item, samples, sampleRate)
			if err != nil {
				return nil, fmt.Errorf("analyze WORLD unit %q: %w", item.Source, err)
			}
			entry = cachedWorldUnit{features: features, duration: duration}
			cache.put(key, entry)
		}
		if fftSize == 0 {
			fftSize = entry.features.FFTSize
		}
		if entry.features.FFTSize != fftSize {
			return nil, fmt.Errorf("WORLD units have inconsistent FFT sizes")
		}
		prepared[index] = preparedWorldUnit{cached: entry}
	}
	bins := fftSize/2 + 1
	result := worldFeatures{
		Frames: frames, FFTSize: fftSize, F0: append([]float64(nil), input.F0Curve...),
		Spectrum: make([]float64, frames*bins), Aperiodicity: make([]float64, frames*bins),
	}
	for index := range result.Spectrum {
		result.Spectrum[index] = 1e-12
		result.Aperiodicity[index] = 1
	}
	dirty := make([]bool, frames)
	sourceVoiced := make([]bool, frames)
	for unitIndex, item := range input.Units {
		entry := prepared[unitIndex].cached
		for frame := 0; frame < frames; frame++ {
			timeMS := float64(frame) * worldFramePeriodMS
			localMS := timeMS - item.PositionMS
			if localMS < 0 || localMS > item.LengthMS {
				continue
			}
			weight := worldEnvelopeWeight(item, localMS)
			if weight <= 1e-6 {
				continue
			}
			volumeGain := math.Max(0, item.Volume) / 100
			sourceMS := mapWorldSourceTime(item, entry.duration, localMS)
			sourceFrame := sourceMS / worldFramePeriodMS
			left := min(max(0, int(math.Floor(sourceFrame))), entry.features.Frames-1)
			right := min(left+1, entry.features.Frames-1)
			fraction := sourceFrame - float64(left)
			voicedFrame := lerp(entry.features.F0[left], entry.features.F0[right], fraction) > 71
			if !dirty[frame] || weight > 0.5 {
				sourceVoiced[frame] = voicedFrame
			}
			for bin := 0; bin < bins; bin++ {
				leftIndex, rightIndex := left*bins+bin, right*bins+bin
				spectrum := lerp(entry.features.Spectrum[leftIndex], entry.features.Spectrum[rightIndex], fraction) * volumeGain * volumeGain
				ap := lerp(entry.features.Aperiodicity[leftIndex], entry.features.Aperiodicity[rightIndex], fraction)
				result.Spectrum[frame*bins+bin] += weight * spectrum
				if !dirty[frame] {
					result.Aperiodicity[frame*bins+bin] = ap
				} else {
					result.Aperiodicity[frame*bins+bin] = result.Aperiodicity[frame*bins+bin]*(1-weight) + ap*weight
				}
			}
			dirty[frame] = true
		}
	}
	for frame := 0; frame < frames; frame++ {
		if !dirty[frame] {
			result.F0[frame] = 0
			continue
		}
		if !sourceVoiced[frame] {
			result.F0[frame] = 0
		}
	}
	wave, err := engine.Synthesize(result, input.SampleRate)
	if err != nil {
		return nil, err
	}
	output := make([]float32, len(wave))
	for index, sample := range wave {
		output[index] = float32(sample)
	}
	return output, nil
}

func analyzeWorldUnit(engine worldEngine, item unit, samples []float64, sampleRate int) (worldFeatures, float64, error) {
	hopSize := int(math.Round(worldFramePeriodMS * float64(sampleRate) / 1000))
	fullFrames := len(samples)/hopSize + 1
	startFrame := max(0, int(item.OffsetMS/worldFramePeriodMS))
	endMS := float64(len(samples))*1000/float64(sampleRate) - item.CutoffMS
	if item.CutoffMS < 0 {
		endMS = item.OffsetMS - item.CutoffMS
	}
	endFrame := min(fullFrames, int(math.Ceil(endMS/worldFramePeriodMS)))
	if endFrame <= startFrame || endFrame-startFrame < 2 {
		return worldFeatures{}, 0, fmt.Errorf("usable source region is too short")
	}
	trimStart := max(0, startFrame-2)
	trimEnd := min(fullFrames, endFrame+2)
	trimmed := make([]float64, (trimEnd-trimStart)*hopSize)
	sampleStart := trimStart * hopSize
	if sampleStart < len(samples) {
		copy(trimmed, samples[sampleStart:min(len(samples), trimEnd*hopSize)])
	}

	var inputF0 []float64
	if item.FrqPath != "" {
		if frq, err := readWorldFRQ(item.FrqPath); err == nil {
			fullF0 := sampleWorldFRQ(frq, fullFrames, hopSize, 71)
			inputF0 = append([]float64(nil), fullF0[trimStart:trimEnd]...)
		}
	}
	features, err := engine.Analyze(trimmed, sampleRate, inputF0)
	if err != nil {
		return worldFeatures{}, 0, err
	}

	left := startFrame - trimStart
	length := endFrame - startFrame
	features, err = sliceWorldFeatures(features, left, length)
	if err != nil {
		return worldFeatures{}, 0, err
	}
	gain := worldAutoGain(trimmed, samples, features.F0)
	for index := range features.Spectrum {
		features.Spectrum[index] *= gain * gain
	}
	return features, float64(length) * worldFramePeriodMS, nil
}

func sliceWorldFeatures(features worldFeatures, start, length int) (worldFeatures, error) {
	if start < 0 || length < 2 || start+length > features.Frames {
		return worldFeatures{}, fmt.Errorf("WORLD feature slice is outside analysis: %d+%d > %d", start, length, features.Frames)
	}
	bins := features.FFTSize/2 + 1
	return worldFeatures{
		Frames: length, FFTSize: features.FFTSize,
		F0:           append([]float64(nil), features.F0[start:start+length]...),
		Spectrum:     append([]float64(nil), features.Spectrum[start*bins:(start+length)*bins]...),
		Aperiodicity: append([]float64(nil), features.Aperiodicity[start*bins:(start+length)*bins]...),
	}, nil
}

func worldAutoGain(segment, source, f0 []float64) float64 {
	maxAbs := func(values []float64) float64 {
		var result float64
		for _, value := range values {
			result = math.Max(result, math.Abs(value))
		}
		return result
	}
	var voiced int
	for _, value := range f0 {
		if value > 71 {
			voiced++
		}
	}
	voicedRatio := float64(voiced) / float64(max(1, len(f0)))
	weight := 1 / (1 + math.Exp(5-10*voicedRatio))
	peak := maxAbs(segment)*weight + maxAbs(source)*(1-weight)
	if peak < 1e-3 {
		return 1
	}
	return math.Pow(0.5/peak, 0.86)
}

func mapWorldSourceTime(item unit, sourceDuration, localMS float64) float64 {
	destinationMS := math.Max(0, item.SkipMS+localMS)
	consonantSpeed := math.Pow(0.5, 1-item.ConsonantVelocity/100)
	sourceConsonant := min(math.Max(0, item.ConsonantMS), sourceDuration)
	destinationConsonant := sourceConsonant / consonantSpeed
	if destinationMS < destinationConsonant {
		return min(sourceDuration, destinationMS*consonantSpeed)
	}
	destinationVowel := math.Max(worldFramePeriodMS, item.RequiredLengthMS-destinationConsonant)
	sourceVowel := math.Max(0, sourceDuration-sourceConsonant)
	return min(sourceDuration, sourceConsonant+(destinationMS-destinationConsonant)*sourceVowel/destinationVowel)
}

func worldEnvelopeWeight(item unit, localMS float64) float64 {
	weight := 1.0
	if item.FadeInMS > 0 && localMS < item.FadeInMS {
		weight = localMS / item.FadeInMS
	}
	remaining := item.LengthMS - localMS
	if item.FadeOutMS > 0 && remaining < item.FadeOutMS {
		weight = math.Min(weight, remaining/item.FadeOutMS)
	}
	return weight
}

func lerp(left, right, fraction float64) float64 {
	return left + (right-left)*math.Max(0, math.Min(1, fraction))
}
