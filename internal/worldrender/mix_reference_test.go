package worldrender

import (
	"math"
	"testing"
)

func TestWorldPreparedMixFixedFeatures(t *testing.T) {
	for _, tc := range []struct {
		name     string
		input    phraseInput
		prepared []preparedWorldUnit
		want     worldFeatures
	}{
		{
			name: "explicit anchors include offset once",
			input: phraseInput{F0Curve: []float64{180, 190, 200}, Units: []Unit{{
				LengthMS: 20, RequiredLengthMS: 20, OffsetMS: 5,
				Speech: &SpeechTiming{Anchors: []SpeechAnchor{
					{SourceMS: 0, TargetMS: 0}, {SourceMS: 5, TargetMS: 10}, {SourceMS: 20, TargetMS: 20},
				}},
			}}},
			prepared: []preparedWorldUnit{{cached: cachedWorldUnit{
				duration: 30, sourceShiftMS: 5, features: worldFeatures{
					Frames: 4, FFTSize: 2, F0: []float64{200, 200, 200, 200},
					Spectrum:     []float64{2, 4, 4, 8, 6, 12, 8, 16},
					Aperiodicity: []float64{0, 0, 0, 0, 0, 0, 0, 0},
				},
			}}},
			// The two anchor segments give source times of 5, 10, 25 ms.
			want: worldFeatures{Frames: 3, FFTSize: 2, F0: []float64{180, 190, 200},
				Spectrum: []float64{3, 6, 4, 8, 7, 14}, Aperiodicity: []float64{0, 0, 0, 0, 0, 0}},
		},
		{
			name: "distinct unit preparation with fractional offsets",
			input: phraseInput{F0Curve: []float64{180, 190, 200, 210}, Units: []Unit{
				{LengthMS: 10, RequiredLengthMS: 30, ConsonantVelocity: 100, OffsetMS: 2.5},
				{PositionMS: 20, LengthMS: 10, RequiredLengthMS: 30, Volume: 50, OffsetMS: 5,
					Speech: &SpeechTiming{Anchors: []SpeechAnchor{{SourceMS: 0, TargetMS: 0}, {SourceMS: 10, TargetMS: 20}}}},
			}},
			prepared: []preparedWorldUnit{
				{cached: cachedWorldUnit{duration: 30, sourceShiftMS: 2.5, features: worldFeatures{
					Frames: 4, FFTSize: 2, F0: []float64{200, 200, 200, 200},
					Spectrum:     []float64{2, 4, 4, 8, 6, 12, 8, 16},
					Aperiodicity: []float64{0, 0, 0, 0, 0, 0, 0, 0},
				}}},
				{cached: cachedWorldUnit{duration: 30, sourceShiftMS: 5, features: worldFeatures{
					Frames: 4, FFTSize: 2, F0: []float64{200, 200, 200, 200},
					Spectrum:     []float64{10, 20, 14, 28, 18, 36, 22, 44},
					Aperiodicity: []float64{0, 0, 0, 0, 0, 0, 0, 0},
				}}},
			},
			// Unit 0 samples 2.5, 12.5 ms; unit 1 samples 5, 10 ms at half amplitude.
			want: worldFeatures{Frames: 4, FFTSize: 2, F0: []float64{180, 190, 200, 210},
				Spectrum: []float64{2.5, 5, 4.5, 9, 3, 6, 3.5, 7}, Aperiodicity: []float64{0, 0, 0, 0, 0, 0, 0, 0}},
		},
		{
			name: "voicing uses gain squared rather than spectral power",
			input: phraseInput{F0Curve: []float64{180, 190, 200}, Units: []Unit{
				{LengthMS: 20, RequiredLengthMS: 20},
				{LengthMS: 20, RequiredLengthMS: 20, Volume: 50},
			}},
			prepared: []preparedWorldUnit{
				{cached: cachedWorldUnit{duration: 20, features: worldFeatures{
					Frames: 3, FFTSize: 2, F0: []float64{200, 71, 200},
					Spectrum: []float64{1, 1, 1, 1, 1, 1}, Aperiodicity: []float64{.5, .5, .5, .5, .5, .5},
				}}},
				{cached: cachedWorldUnit{duration: 20, features: worldFeatures{
					Frames: 3, FFTSize: 2, F0: []float64{0, 200, 0},
					Spectrum: []float64{16, 16, 16, 16, 16, 16}, Aperiodicity: []float64{.25, .25, .25, .25, .25, .25},
				}}},
			},
			// Voicing weights are 1 and 1/4; spectral powers are 1 and 4.
			// Unvoiced AP becomes 1: AP power ratios are 4.25/5, 1.25/5, 4.25/5.
			want: worldFeatures{Frames: 3, FFTSize: 2, F0: []float64{180, 0, 200},
				Spectrum:     []float64{2.5, 2.5, 2.5, 2.5, 2.5, 2.5},
				Aperiodicity: []float64{math.Sqrt(.85), math.Sqrt(.85), .5, .5, math.Sqrt(.85), math.Sqrt(.85)}},
		},
		{
			name: "equal voiced and unvoiced weight is unvoiced",
			input: phraseInput{F0Curve: []float64{180, 190}, Units: []Unit{
				{LengthMS: 10, RequiredLengthMS: 10}, {LengthMS: 10, RequiredLengthMS: 10},
			}},
			prepared: []preparedWorldUnit{
				{cached: cachedWorldUnit{duration: 10, features: worldFeatures{
					Frames: 2, FFTSize: 2, F0: []float64{200, 0},
					Spectrum: []float64{3, 3, 3, 3}, Aperiodicity: []float64{0, 0, 0, 0},
				}}},
				{cached: cachedWorldUnit{duration: 10, features: worldFeatures{
					Frames: 2, FFTSize: 2, F0: []float64{0, 200},
					Spectrum: []float64{1, 1, 1, 1}, Aperiodicity: []float64{0, 0, 0, 0},
				}}},
			},
			want: worldFeatures{Frames: 2, FFTSize: 2, F0: []float64{0, 0},
				Spectrum: []float64{2, 2, 2, 2}, Aperiodicity: []float64{.5, .5, math.Sqrt(.75), math.Sqrt(.75)}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := mixWorldFeatures(tc.input, tc.prepared, 2, 1)
			if got.Frames != tc.want.Frames || got.FFTSize != tc.want.FFTSize {
				t.Fatalf("feature dimensions = %d/%d, want %d/%d", got.Frames, got.FFTSize, tc.want.Frames, tc.want.FFTSize)
			}
			for _, field := range []struct {
				name      string
				got, want []float64
			}{{"F0", got.F0, tc.want.F0}, {"spectrum", got.Spectrum, tc.want.Spectrum}, {"aperiodicity", got.Aperiodicity, tc.want.Aperiodicity}} {
				if len(field.got) != len(field.want) {
					t.Fatalf("%s length = %d, want %d", field.name, len(field.got), len(field.want))
				}
				for i, want := range field.want {
					if math.IsNaN(field.got[i]) || math.Abs(field.got[i]-want) > 1e-9 {
						t.Errorf("%s[%d] = %g, want %g", field.name, i, field.got[i], want)
					}
				}
			}
		})
	}
}
