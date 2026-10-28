package speechtiming

import (
	"math"
	"reflect"
	"testing"
)

func resampleFixture() Features {
	const frames, fftSize = 320, 2048
	bins := fftSize/2 + 1
	f := Features{Frames: frames, FFTSize: fftSize, SampleRate: 44100, F0: make([]float64, frames), Spectrum: make([]float64, frames*bins), Aperiodicity: make([]float64, frames*bins)}
	for frame := range frames {
		if frame%5 != 0 {
			f.F0[frame] = 180 + float64(frame%17)
		}
		for bin := range bins {
			f.Spectrum[frame*bins+bin] = float64((frame+bin)%37) / 10
			f.Aperiodicity[frame*bins+bin] = float64((frame+bin)%19) / 10
		}
	}
	return f
}

func TestResampleReusesLogsWithoutChangingFeatures(t *testing.T) {
	input := Features{
		Frames: 7, FFTSize: 2, SampleRate: 44100,
		F0: []float64{100, 0, 400, 0, 1600, 0, 6400},
		Spectrum: []float64{
			1, 4,
			4, 16,
			16, 64,
			64, 256,
			256, 1024,
			1024, 4096,
			4096, 16384,
		},
		Aperiodicity: []float64{
			-1, .25,
			0, 1,
			1, 4,
			4, .0625,
			.0625, .25,
			.25, 1,
			1, 4,
		},
	}
	before := input
	before.F0 = append([]float64(nil), input.F0...)
	before.Spectrum = append([]float64(nil), input.Spectrum...)
	before.Aperiodicity = append([]float64(nil), input.Aperiodicity...)
	positions := []float64{.5, .5, 4.5, 1.5, .5, 5.5, 6}
	beforePositions := append([]float64(nil), positions...)
	output := resample(input, positions)
	if output.Frames != 7 || output.FFTSize != 2 || output.SampleRate != 44100 {
		t.Fatal("feature dimensions changed")
	}
	for _, pair := range []struct {
		name      string
		got, want []float64
	}{
		{"F0", output.F0, []float64{0, 0, 0, 800, 0, 3200, 6400}},
		{"spectrum", output.Spectrum, []float64{
			2, 8,
			2, 8,
			512, 2048,
			8, 32,
			2, 8,
			2048, 8192,
			4096, 16384,
		}},
		{"aperiodicity", output.Aperiodicity, []float64{
			1e-16, .5,
			1e-16, .5,
			.125, .5,
			1e-8, 1,
			1e-16, .5,
			.5, 1,
			1, 1,
		}},
	} {
		if len(pair.got) != len(pair.want) {
			t.Fatalf("%s length changed: %d != %d", pair.name, len(pair.got), len(pair.want))
		}
		for i, want := range pair.want {
			if !resampledValueClose(pair.got[i], want) {
				t.Fatalf("%s[%d]: got %.17g, want %.17g", pair.name, i, pair.got[i], want)
			}
		}
	}
	if !reflect.DeepEqual(input, before) || !reflect.DeepEqual(positions, beforePositions) {
		t.Fatal("input mutated")
	}
}

func resampledValueClose(got, want float64) bool {
	if math.IsNaN(got) || math.IsNaN(want) || math.IsInf(got, 0) || math.IsInf(want, 0) {
		return false
	}
	// 積和演算の融合による丸め差を許容する。
	return math.Abs(got-want) <= 1e-13*math.Abs(want)
}

func BenchmarkResampleFeatures(b *testing.B) {
	input := resampleFixture()
	positions := make([]float64, input.Frames)
	for frame := range positions {
		positions[frame] = float64(frame) * .8
	}
	b.ReportAllocs()
	for b.Loop() {
		resample(input, positions)
	}
}
