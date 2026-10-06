package worldrender

import (
	"math"
	"path/filepath"
	"testing"

	"utautts/internal/provider"
)

type fakeWorldEngine struct {
	analyzeCalls int
	synthCalls   int
}

func (e *fakeWorldEngine) Close() error { return nil }

func (e *fakeWorldEngine) Analyze(samples []float64, rate int, f0 []float64) (worldFeatures, error) {
	e.analyzeCalls++
	hop := int(math.Round(worldFramePeriodMS * float64(rate) / 1000))
	frames := len(samples)/hop + 1
	const fftSize = 16
	bins := fftSize/2 + 1
	features := worldFeatures{Frames: frames, FFTSize: fftSize,
		F0: make([]float64, frames), Spectrum: make([]float64, frames*bins), Aperiodicity: make([]float64, frames*bins)}
	for frame := 0; frame < frames; frame++ {
		features.F0[frame] = 200
		for bin := 0; bin < bins; bin++ {
			features.Spectrum[frame*bins+bin] = 1
			features.Aperiodicity[frame*bins+bin] = .1
		}
	}
	return features, nil
}

func (e *fakeWorldEngine) Synthesize(features worldFeatures, rate int) ([]float64, error) {
	e.synthCalls++
	return make([]float64, worldSynthesisLength(features.Frames, rate)), nil
}

func TestWorldPhrasePipelineRunsStagesInOrder(t *testing.T) {
	const rate = 16000
	path := filepath.Join(t.TempDir(), "unit.wav")
	samples := make([]float32, rate)
	for index := range samples {
		samples[index] = .1 * float32(math.Sin(2*math.Pi*200*float64(index)/rate))
	}
	if err := writePCM16(path, rate, samples); err != nil {
		t.Fatal(err)
	}
	engine := &fakeWorldEngine{}
	var results []provider.WorldSpeechResult
	input := manifest{
		Engine: "utautts-world-phrase", SampleRate: rate,
		F0Curve:       []float64{200, 200, 200, 200, 200, 200, 200, 200},
		SpeechResults: &results,
		Units: []unit{{
			Source: path, PositionMS: 0, LengthMS: 80, RequiredLengthMS: 80,
			Volume: 100, VolumeSet: true, Speech: &provider.WorldSpeechTiming{UnitIndex: 0},
		}},
	}
	output, err := renderUtauTTSWorldPhrase(engine, input, newWorldFeatureCache(4))
	if err != nil {
		t.Fatal(err)
	}
	if engine.analyzeCalls != 1 || engine.synthCalls != 1 {
		t.Fatalf("analyze=%d synth=%d, want 1/1", engine.analyzeCalls, engine.synthCalls)
	}
	if len(output) != worldSynthesisLength(len(input.F0Curve), rate) {
		t.Fatalf("output length = %d", len(output))
	}
	if len(results) != 1 || results[0].UnitIndex != 0 {
		t.Fatalf("speech results = %+v", results)
	}
}

func TestWorldPhrasePipelineRejectsShortPhrase(t *testing.T) {
	if _, err := renderUtauTTSWorldPhrase(&fakeWorldEngine{}, manifest{F0Curve: []float64{200}}, nil); err == nil {
		t.Fatal("short phrase was accepted")
	}
}
