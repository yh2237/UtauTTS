package worldrender

import (
	"math"
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/yh2237/UtauTTS/internal/provider"
)

func mixWorldFeaturesReference(input manifest, prepared []preparedWorldUnit, fftSize, workers int) worldFeatures {
	frames, bins := len(input.F0Curve), fftSize/2+1
	result := worldFeatures{Frames: frames, FFTSize: fftSize, F0: append([]float64(nil), input.F0Curve...),
		Spectrum: make([]float64, frames*bins), Aperiodicity: make([]float64, frames*bins)}
	dirty := make([]bool, frames)
	if frames*bins < 32768 {
		workers = 1
	}
	parallelWorldWork(frames, workers, func(frame int) {
		voicedWeight, totalWeight, envelopeWeight := 0., 0., 0.
		normalizeOverlap := false
		frameOffset := frame * bins
		for bin := 0; bin < bins; bin++ {
			result.Spectrum[frameOffset+bin] = 1e-12
		}
		for index, item := range input.Units {
			entry := prepared[index].cached
			localMS := float64(frame)*worldFramePeriodMS - item.PositionMS
			if localMS < 0 || localMS > item.LengthMS {
				continue
			}
			weight := worldEnvelopeWeight(item, localMS)
			if weight <= 1e-6 {
				continue
			}
			if !item.LegacyMix {
				normalizeOverlap = true
			}
			gain := worldUnitAmplitudeGain(item)
			sourceFrame := mapWorldFeatureTime(item, entry, localMS) / worldFramePeriodMS
			left := min(max(0, int(math.Floor(sourceFrame))), entry.features.Frames-1)
			right, fraction := min(left+1, entry.features.Frames-1), sourceFrame-float64(left)
			voiced := lerp(entry.features.F0[left], entry.features.F0[right], fraction) > 71
			effectiveWeight := weight * gain * gain
			envelopeWeight += weight
			totalWeight += effectiveWeight
			if voiced {
				voicedWeight += effectiveWeight
			}
			for bin := 0; bin < bins; bin++ {
				li, ri := left*bins+bin, right*bins+bin
				spectrum := lerp(entry.features.Spectrum[li], entry.features.Spectrum[ri], fraction) * gain * gain
				ap := lerp(entry.features.Aperiodicity[li], entry.features.Aperiodicity[ri], fraction)
				if !voiced {
					ap = 1
				}
				result.Spectrum[frameOffset+bin] += weight * spectrum
				result.Aperiodicity[frameOffset+bin] += weight * spectrum * ap * ap
			}
			dirty[frame] = true
		}
		if normalizeOverlap && envelopeWeight > 1 {
			normalization := 1 / envelopeWeight
			for bin := 0; bin < bins; bin++ {
				result.Spectrum[frameOffset+bin] *= normalization
				result.Aperiodicity[frameOffset+bin] *= normalization
			}
		}
		for bin := 0; bin < bins; bin++ {
			i := frameOffset + bin
			if !dirty[frame] || totalWeight <= 1e-12 {
				result.Aperiodicity[i] = 1
			} else {
				result.Aperiodicity[i] = math.Sqrt(math.Max(0, math.Min(1, result.Aperiodicity[i]/result.Spectrum[i])))
			}
		}
		if totalWeight <= 1e-12 || voicedWeight*2 <= totalWeight {
			result.F0[frame] = 0
		}
	})
	for frame := range dirty {
		if !dirty[frame] {
			result.F0[frame] = 0
		}
	}
	return result
}

func TestWorldPreparedMixMatchesReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 42))
	input := manifest{F0Curve: make([]float64, 600)}
	for frame := range input.F0Curve {
		input.F0Curve[frame] = 200
	}
	for range 100 {
		input.Units = append(input.Units, unit{PositionMS: rng.Float64()*6000 - 100, LengthMS: rng.Float64() * 200,
			RequiredLengthMS: 120, ConsonantVelocity: 86, ConsonantMS: 10, FadeInMS: 15, FadeOutMS: 20,
			Volume: rng.Float64() * 140, VolumeSet: true, EnergyFactor: .7 + rng.Float64()*.6, LegacyMix: rng.IntN(2) == 0})
	}
	prepared := flatPreparedUnits(len(input.Units))
	for i := range input.Units {
		if i%3 == 0 {
			input.Units[i].Speech = &provider.WorldSpeechTiming{Anchors: []provider.SpeechAnchor{
				{SourceMS: 0, TargetMS: 0}, {SourceMS: 8, TargetMS: 10}, {SourceMS: 18, TargetMS: 120}}}
		}
		prepared[i].cached.sourceShiftMS = .5
		prepared[i].cached.features.F0[0] = float64(i%2) * 200
	}
	want := mixWorldFeaturesReference(input, prepared, 2, 1)
	for _, workers := range []int{1, 4} {
		if !reflect.DeepEqual(mixWorldFeatures(input, prepared, 2, workers), want) {
			t.Fatal("mixed features changed")
		}
	}
}
