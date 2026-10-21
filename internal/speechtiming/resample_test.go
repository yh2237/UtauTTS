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
	input := resampleFixture()
	beforeSP, beforeAP := append([]float64(nil), input.Spectrum...), append([]float64(nil), input.Aperiodicity...)
	positions := make([]float64, input.Frames)
	for frame := range positions {
		positions[frame] = float64(frame%127) * 1.7
	}
	positions[0], positions[1], positions[2] = 0, 0, float64(input.Frames-1)
	output := resample(input, positions)
	reference := resampleReference(input, positions)
	if output.Frames != reference.Frames || output.FFTSize != reference.FFTSize || output.SampleRate != reference.SampleRate {
		t.Fatal("feature dimensions changed")
	}
	if !reflect.DeepEqual(output.F0, reference.F0) {
		t.Fatal("F0 changed")
	}
	for _, pair := range []struct {
		name      string
		got, want []float64
	}{{"spectrum", output.Spectrum, reference.Spectrum}, {"aperiodicity", output.Aperiodicity, reference.Aperiodicity}} {
		if len(pair.got) != len(pair.want) {
			t.Fatalf("%s length changed: %d != %d", pair.name, len(pair.got), len(pair.want))
		}
		for i, want := range pair.want {
			if !resampledValueClose(pair.got[i], want) {
				t.Fatalf("%s changed at frame %d bin %d: got %.17g, want %.17g", pair.name, i/(input.FFTSize/2+1), i%(input.FFTSize/2+1), pair.got[i], want)
			}
		}
	}
	if !reflect.DeepEqual(input.Spectrum, beforeSP) || !reflect.DeepEqual(input.Aperiodicity, beforeAP) {
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

func resampleReference(input Features, positions []float64) Features {
	frames, bins := input.Frames, input.FFTSize/2+1
	output := Features{Frames: frames, FFTSize: input.FFTSize, SampleRate: input.SampleRate,
		F0: make([]float64, frames), Spectrum: make([]float64, frames*bins), Aperiodicity: make([]float64, frames*bins)}
	var voiced []int
	for t, value := range input.F0 {
		if value > 0 {
			voiced = append(voiced, t)
		}
	}
	for t, position := range positions {
		left := int(math.Floor(position))
		right, w := min(left+1, frames-1), position-float64(left)
		for bin := range bins {
			for _, pair := range [][2][]float64{{input.Spectrum, output.Spectrum}, {input.Aperiodicity, output.Aperiodicity}} {
				a := math.Log(math.Max(pair[0][left*bins+bin], 1e-16))
				b := math.Log(math.Max(pair[0][right*bins+bin], 1e-16))
				pair[1][t*bins+bin] = math.Exp(a*(1-w) + b*w)
			}
			output.Aperiodicity[t*bins+bin] = math.Min(output.Aperiodicity[t*bins+bin], 1)
		}
		nearest := min(max(int(math.Round(position)), 0), frames-1)
		if input.F0[nearest] > 0 && len(voiced) > 0 {
			if input.F0[t] > 0 {
				output.F0[t] = input.F0[t]
			} else {
				output.F0[t] = math.Exp(interpLogF0(input.F0, voiced, t, 0))
			}
		}
	}
	return output
}

func BenchmarkResampleFeatures(b *testing.B) {
	input := resampleFixture()
	positions := make([]float64, input.Frames)
	for frame := range positions {
		positions[frame] = float64(frame) * .8
	}
	for _, tc := range []struct {
		name string
		run  func(Features, []float64) Features
	}{{"reference", resampleReference}, {"cached", resample}} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				tc.run(input, positions)
			}
		})
	}
}
