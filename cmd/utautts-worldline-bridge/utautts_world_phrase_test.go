package main

import (
	"math"
	"reflect"
	"testing"

	"utautts/internal/provider"
)

func TestWorldMixIsOrderIndependentAndPreservesFade(t *testing.T) {
	makeUnit := func(f0, ap float64) preparedWorldUnit {
		return preparedWorldUnit{cached: cachedWorldUnit{duration: 20, features: worldFeatures{Frames: 2, FFTSize: 2, F0: []float64{f0, f0}, Spectrum: []float64{1, 1, 1, 1}, Aperiodicity: []float64{ap, ap, ap, ap}}}}
	}
	u := unit{LengthMS: 40, FadeInMS: 40, RequiredLengthMS: 40, ConsonantVelocity: 100, Volume: 100}
	in := manifest{F0Curve: []float64{200, 200, 200}, Units: []unit{u, u}}
	a, b := makeUnit(200, .1), makeUnit(0, .9)
	ab := mixWorldFeatures(in, []preparedWorldUnit{a, b}, 2, 1)
	ba := mixWorldFeatures(in, []preparedWorldUnit{b, a}, 2, 1)
	if !reflect.DeepEqual(ab, ba) {
		t.Fatalf("order-dependent mix: %+v / %+v", ab, ba)
	}
	if math.Abs(ab.Spectrum[2]-.5) > 1e-10 {
		t.Fatal("fade was normalized away")
	}
	if math.Abs(ab.Aperiodicity[2]-math.Sqrt(.505)) > 1e-10 {
		t.Fatal("aperiodic energy was not conserved")
	}
}

func TestWorldMixNormalizesOverlappingEnvelopeGain(t *testing.T) {
	prepared := []preparedWorldUnit{{cached: cachedWorldUnit{duration: 20, features: worldFeatures{
		Frames: 2, FFTSize: 2, F0: []float64{200, 200}, Spectrum: []float64{1, 1, 1, 1}, Aperiodicity: []float64{.2, .2, .2, .2},
	}}}, {cached: cachedWorldUnit{duration: 20, features: worldFeatures{
		Frames: 2, FFTSize: 2, F0: []float64{200, 200}, Spectrum: []float64{1, 1, 1, 1}, Aperiodicity: []float64{.2, .2, .2, .2},
	}}}}
	input := manifest{F0Curve: []float64{200, 200}, Units: []unit{
		{LengthMS: 20, RequiredLengthMS: 20, Volume: 100},
		{LengthMS: 20, RequiredLengthMS: 20, Volume: 100},
	}}
	result := mixWorldFeatures(input, prepared, 2, 1)
	if math.Abs(result.Spectrum[0]-1) > 1e-9 {
		t.Fatalf("overlap gain = %f, want 1", result.Spectrum[0])
	}
	if math.Abs(result.Aperiodicity[0]-.2) > 1e-9 {
		t.Fatalf("overlap aperiodicity = %f, want .2", result.Aperiodicity[0])
	}
}

func TestWorldMixLegacyModeKeepsOverlappingGain(t *testing.T) {
	prepared := []preparedWorldUnit{{cached: cachedWorldUnit{duration: 20, features: worldFeatures{
		Frames: 2, FFTSize: 2, F0: []float64{200, 200}, Spectrum: []float64{1, 1, 1, 1}, Aperiodicity: []float64{.2, .2, .2, .2},
	}}}, {cached: cachedWorldUnit{duration: 20, features: worldFeatures{
		Frames: 2, FFTSize: 2, F0: []float64{200, 200}, Spectrum: []float64{1, 1, 1, 1}, Aperiodicity: []float64{.2, .2, .2, .2},
	}}}}
	input := manifest{F0Curve: []float64{200, 200}, Units: []unit{
		{LengthMS: 20, RequiredLengthMS: 20, Volume: 100, LegacyMix: true},
		{LengthMS: 20, RequiredLengthMS: 20, Volume: 100, LegacyMix: true},
	}}
	result := mixWorldFeatures(input, prepared, 2, 1)
	if math.Abs(result.Spectrum[0]-2) > 1e-9 {
		t.Fatalf("legacy overlap gain = %f, want 2", result.Spectrum[0])
	}
	if math.Abs(result.Aperiodicity[0]-.2) > 1e-9 {
		t.Fatalf("legacy overlap aperiodicity = %f, want .2", result.Aperiodicity[0])
	}
}

func TestWorldEnvelopeUsesLinearFades(t *testing.T) {
	item := unit{LengthMS: 200, FadeInMS: 50, FadeOutMS: 50}
	if got := worldEnvelopeWeight(item, 25); math.Abs(got-0.5) > 1e-9 {
		t.Fatalf("fade-in weight = %f, want 0.5", got)
	}
}

func TestWorldEnvelopeUsesUTAUPoints(t *testing.T) {
	item := unit{LengthMS: 150, Envelope: []envelopePoint{
		{XMS: -100, Y: 0}, {XMS: -50, Y: 1}, {XMS: 0, Y: 1}, {XMS: 50, Y: 0},
	}}
	if got := worldEnvelopeWeight(item, 0); got != 0 {
		t.Fatalf("envelope start = %f, want 0", got)
	}
	if got := worldEnvelopeWeight(item, 75); got != 1 {
		t.Fatalf("envelope plateau = %f, want 1", got)
	}
	if got := worldEnvelopeWeight(item, 125); math.Abs(got-.5) > 1e-9 {
		t.Fatalf("envelope release = %f, want .5", got)
	}
}

func TestWorldEnvelopeLegacyModeUsesLinearFades(t *testing.T) {
	item := unit{LengthMS: 150, FadeInMS: 50, Envelope: []envelopePoint{
		{XMS: -100, Y: 0}, {XMS: -50, Y: 1}, {XMS: 0, Y: 1}, {XMS: 50, Y: 0},
	}, LegacyMix: true}
	if got := worldEnvelopeWeight(item, 25); math.Abs(got-.5) > 1e-9 {
		t.Fatalf("legacy envelope weight = %f, want .5", got)
	}
}

func TestWorldMixAppliesEnergyFactorAsAmplitude(t *testing.T) {
	prepared := []preparedWorldUnit{{cached: cachedWorldUnit{duration: 20, features: worldFeatures{
		Frames: 2, FFTSize: 2, F0: []float64{200, 200}, Spectrum: []float64{1, 1, 1, 1}, Aperiodicity: []float64{.2, .2, .2, .2},
	}}}}
	input := manifest{F0Curve: []float64{200, 200}, Units: []unit{{LengthMS: 20, RequiredLengthMS: 20, Volume: 100, EnergyFactor: .5}}}
	result := mixWorldFeatures(input, prepared, 2, 1)
	if math.Abs(result.Spectrum[0]-.25) > 1e-9 {
		t.Fatalf("energy-scaled spectrum = %f, want .25", result.Spectrum[0])
	}
}

func TestWorldMixNormalizesEnvelopeBeforeEnergyScaling(t *testing.T) {
	prepared := []preparedWorldUnit{{cached: cachedWorldUnit{duration: 20, features: worldFeatures{
		Frames: 2, FFTSize: 2, F0: []float64{200, 200}, Spectrum: []float64{1, 1, 1, 1}, Aperiodicity: []float64{.2, .2, .2, .2},
	}}}, {cached: cachedWorldUnit{duration: 20, features: worldFeatures{
		Frames: 2, FFTSize: 2, F0: []float64{200, 200}, Spectrum: []float64{1, 1, 1, 1}, Aperiodicity: []float64{.2, .2, .2, .2},
	}}}}
	input := manifest{F0Curve: []float64{200, 200}, Units: []unit{
		{LengthMS: 20, RequiredLengthMS: 20, Volume: 100, EnergyFactor: .5},
		{LengthMS: 20, RequiredLengthMS: 20, Volume: 100, EnergyFactor: .5},
	}}
	result := mixWorldFeatures(input, prepared, 2, 1)
	if math.Abs(result.Spectrum[0]-.25) > 1e-9 {
		t.Fatalf("energy-scaled overlap spectrum = %f, want .25", result.Spectrum[0])
	}
}

func TestWorldUnitAmplitudeGainRestoresNeutralDefaults(t *testing.T) {
	if got := worldUnitAmplitudeGain(unit{}); math.Abs(got-1) > 1e-9 {
		t.Fatalf("neutral gain = %f, want 1", got)
	}
	if got := worldUnitAmplitudeGain(unit{Volume: 50, EnergyFactor: .8}); math.Abs(got-.4) > 1e-9 {
		t.Fatalf("combined gain = %f, want .4", got)
	}
	if got := worldUnitAmplitudeGain(unit{Volume: 50, EnergyFactor: .8, LegacyMix: true}); math.Abs(got-.5) > 1e-9 {
		t.Fatalf("legacy gain = %f, want .5", got)
	}
}

func TestMapWorldFeatureTimeAppliesFractionalOtoOffset(t *testing.T) {
	entry := cachedWorldUnit{duration: 300, sourceShiftMS: 3}
	item := unit{ConsonantMS: 100, RequiredLengthMS: 100, ConsonantVelocity: 100}
	if got := mapWorldFeatureTime(item, entry, 20); math.Abs(got-23) > 1e-9 {
		t.Fatalf("feature source time = %f, want 23", got)
	}
	item.LegacyMix = true
	if got := mapWorldFeatureTime(item, entry, 20); math.Abs(got-20) > 1e-9 {
		t.Fatalf("legacy feature source time = %f, want 20", got)
	}
	item.ConsonantMS, item.RequiredLengthMS = 70, 180
	item.Speech = &provider.WorldSpeechTiming{SourceOnsetMS: 50, TargetOnsetMS: 50}
	if got := mapWorldFeatureTime(item, entry, 20); math.Abs(got-20) > 1e-9 {
		t.Fatalf("speech-retimed source time = %f, want 20", got)
	}
}

func TestMapWorldSourceTimePreservesConsonantAndStretchesTail(t *testing.T) {
	item := unit{ConsonantMS: 100, RequiredLengthMS: 500, ConsonantVelocity: 100}
	if got := mapWorldSourceTime(item, 300, 80); got != 80 {
		t.Fatalf("consonant time = %f, want 80", got)
	}
	if got := mapWorldSourceTime(item, 300, 300); math.Abs(got-200) > 1e-9 {
		t.Fatalf("stretched tail time = %f, want 200", got)
	}
}

func TestWorldFeatureCacheIsBounded(t *testing.T) {
	cache := newWorldFeatureCache(2)
	cache.put("a", cachedWorldUnit{})
	cache.put("b", cachedWorldUnit{})
	if _, found := cache.get("a"); !found {
		t.Fatal("recent entry was not found")
	}
	cache.put("c", cachedWorldUnit{})
	if _, found := cache.get("b"); found {
		t.Fatal("least recently used entry was not evicted")
	}
}

func TestWorldFeatureCacheAlsoUsesMemoryLimit(t *testing.T) {
	cache := newWorldFeatureCacheWithLimit(10, 100)
	value := cachedWorldUnit{features: worldFeatures{F0: make([]float64, 10)}}
	cache.put("a", value)
	cache.put("b", value)
	if _, found := cache.get("a"); found {
		t.Fatal("entry beyond the memory limit was not evicted")
	}
	if _, found := cache.get("b"); !found {
		t.Fatal("newest entry was evicted")
	}
	if cache.bytes > cache.maxBytes {
		t.Fatalf("cache bytes = %d, limit = %d", cache.bytes, cache.maxBytes)
	}
}

func TestParallelWorldFeatureMixMatchesSequentialMix(t *testing.T) {
	input, prepared, fftSize := worldMixFixture()
	sequential := mixWorldFeatures(input, prepared, fftSize, 1)
	parallel := mixWorldFeatures(input, prepared, fftSize, 4)
	if !reflect.DeepEqual(parallel, sequential) {
		t.Fatal("parallel feature mix differs from sequential mix")
	}
}

func worldMixFixture() (manifest, []preparedWorldUnit, int) {
	const frames, sourceFrames, fftSize = 240, 80, 512
	bins := fftSize/2 + 1
	input := manifest{F0Curve: make([]float64, frames)}
	prepared := make([]preparedWorldUnit, 12)
	for frame := range input.F0Curve {
		input.F0Curve[frame] = 180 + float64(frame%20)
	}
	for unitIndex := range prepared {
		features := worldFeatures{
			Frames: sourceFrames, FFTSize: fftSize,
			F0: make([]float64, sourceFrames), Spectrum: make([]float64, sourceFrames*bins),
			Aperiodicity: make([]float64, sourceFrames*bins),
		}
		for frame := range features.F0 {
			features.F0[frame] = 180 + float64(unitIndex)
		}
		for index := range features.Spectrum {
			features.Spectrum[index] = 0.1 + float64((index+unitIndex)%17)/20
			features.Aperiodicity[index] = 0.05 + float64((index+unitIndex)%11)/20
		}
		input.Units = append(input.Units, unit{
			PositionMS: float64(unitIndex * 160), LengthMS: 800,
			RequiredLengthMS: 800, ConsonantMS: 100, ConsonantVelocity: 100,
			FadeInMS: 40, FadeOutMS: 60, Volume: 100,
		})
		prepared[unitIndex] = preparedWorldUnit{cached: cachedWorldUnit{
			features: features, duration: sourceFrames * worldFramePeriodMS,
		}}
	}
	return input, prepared, fftSize
}
