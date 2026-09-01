package main

import (
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

type parallelTestWorldEngine struct {
	active   atomic.Int32
	maximum  atomic.Int32
	analyses atomic.Int32
}

func (*parallelTestWorldEngine) Close() error { return nil }

func (engine *parallelTestWorldEngine) Analyze(samples []float64, sampleRate int, inputF0 []float64) (worldFeatures, error) {
	engine.analyses.Add(1)
	active := engine.active.Add(1)
	defer engine.active.Add(-1)
	for maximum := engine.maximum.Load(); active > maximum && !engine.maximum.CompareAndSwap(maximum, active); maximum = engine.maximum.Load() {
	}
	time.Sleep(10 * time.Millisecond)
	frames := len(samples)/max(1, sampleRate/100) + 1
	fftSize := 16
	features := worldFeatures{
		Frames: frames, FFTSize: fftSize,
		F0: make([]float64, frames), Spectrum: make([]float64, frames*(fftSize/2+1)),
		Aperiodicity: make([]float64, frames*(fftSize/2+1)),
	}
	for index := range features.F0 {
		features.F0[index] = 220
	}
	for index := range features.Spectrum {
		features.Spectrum[index] = 1
		features.Aperiodicity[index] = 0.2
	}
	return features, nil
}

func (*parallelTestWorldEngine) Synthesize(features worldFeatures, sampleRate int) ([]float64, error) {
	return make([]float64, worldSynthesisLength(features.Frames, sampleRate)), nil
}

func TestWorldEnvelopeUsesLinearFades(t *testing.T) {
	item := unit{LengthMS: 200, FadeInMS: 50, FadeOutMS: 50}
	if got := worldEnvelopeWeight(item, 25); math.Abs(got-0.5) > 1e-9 {
		t.Fatalf("fade-in weight = %f, want 0.5", got)
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

func TestPrepareWorldUnitsAnalyzesCacheMissesInParallel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unit.wav")
	samples := make([]float32, 3200)
	for index := range samples {
		samples[index] = float32(math.Sin(2 * math.Pi * 220 * float64(index) / 16000))
	}
	if err := writePCM16(path, 16000, samples); err != nil {
		t.Fatal(err)
	}
	input := manifest{SampleRate: 16000, Units: make([]unit, 4)}
	for index := range input.Units {
		input.Units[index] = unit{CacheKey: string(rune('a' + index)), Source: path}
	}
	engine := &parallelTestWorldEngine{}
	prepared, err := prepareWorldUnits(engine, input, newWorldFeatureCache(8), 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared) != len(input.Units) {
		t.Fatalf("prepared units = %d, want %d", len(prepared), len(input.Units))
	}
	if engine.analyses.Load() != 4 {
		t.Fatalf("analyses = %d, want 4", engine.analyses.Load())
	}
	if engine.maximum.Load() < 2 {
		t.Fatalf("maximum concurrent analyses = %d, want at least 2", engine.maximum.Load())
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

func BenchmarkWorldFeatureMix(b *testing.B) {
	input, prepared, fftSize := worldMixFixture()
	for _, workers := range []int{1, max(2, runtime.GOMAXPROCS(0))} {
		b.Run(fmt.Sprintf("workers-%d", workers), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = mixWorldFeatures(input, prepared, fftSize, workers)
			}
		})
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
