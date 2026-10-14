package tts

import (
	"math"
	"testing"

	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/prosody"
	"github.com/yh2237/UtauTTS/internal/render"
)

func TestMandarinCorrectionKeepsBudgetAtSharedBoundary(t *testing.T) {
	for _, weight := range []float64{-100, 100} {
		base := &render.PitchCurve{FrameMS: 10, Cents: make([]float64, 21)}
		model := &prosody.MandarinIntonationModel{
			FeatureNames: []string{"bias"}, Knots: []float64{.25, .85},
			Weights: [][]float64{{weight}, {weight}}, Strength: .65, MaxCents: 100,
		}
		morae := []frontend.Mora{{Tone: 1}, {Tone: 2}}
		timings := []prosody.MoraTiming{{StartMS: 0, DurationMS: 100}, {StartMS: 100, DurationMS: 100}}
		got := applyMandarinIntonation(base, model, morae, timings)
		for frame, value := range got.Cents {
			if math.Abs(value-weight*.65) > 1e-10 {
				t.Fatalf("weight %g frame %d: correction = %g, want %g", weight, frame, value, weight*.65)
			}
			if base.Cents[frame] != 0 {
				t.Fatal("base curve was mutated")
			}
		}
	}
}

func TestMandarinCorrectionBoundaryBelongsToFollowingUnit(t *testing.T) {
	model := &prosody.MandarinIntonationModel{
		FeatureNames: []string{"tone_1"}, Knots: []float64{.25, .85},
		Weights: [][]float64{{80}, {80}}, Strength: .5, MaxCents: 100,
	}
	for _, tc := range []struct {
		name    string
		morae   []frontend.Mora
		timings []prosody.MoraTiming
		frames  int
		want    map[int]float64
	}{
		{"shared", []frontend.Mora{{Tone: 1}, {Tone: 2}},
			[]prosody.MoraTiming{{StartMS: 0, DurationMS: 100}, {StartMS: 100, DurationMS: 100}}, 21,
			map[int]float64{9: 40, 10: 0, 11: 0, 20: 0}},
		{"fractional", []frontend.Mora{{Tone: 1}, {Tone: 2}},
			[]prosody.MoraTiming{{StartMS: 0, DurationMS: 100.5}, {StartMS: 100.5, DurationMS: 100.5}}, 22,
			map[int]float64{10: 40, 11: 0}},
		{"gap", []frontend.Mora{{Tone: 1}, {Tone: 2}},
			[]prosody.MoraTiming{{StartMS: 0, DurationMS: 100}, {StartMS: 120, DurationMS: 100}}, 23,
			map[int]float64{10: 40, 11: 0, 12: 0}},
		{"pause", []frontend.Mora{{Tone: 1}, {Pause: true}, {Tone: 2}},
			[]prosody.MoraTiming{{StartMS: 0, DurationMS: 100}, {StartMS: 100, DurationMS: 100}, {StartMS: 200, DurationMS: 100}}, 31,
			map[int]float64{10: 40, 11: 0, 20: 0}},
		{"unknown-tone", []frontend.Mora{{Tone: 1}, {Tone: 0}},
			[]prosody.MoraTiming{{StartMS: 0, DurationMS: 100}, {StartMS: 100, DurationMS: 100}}, 21,
			map[int]float64{10: 40, 11: 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := &render.PitchCurve{FrameMS: 10, Cents: make([]float64, tc.frames)}
			got := applyMandarinIntonation(base, model, tc.morae, tc.timings)
			for frame, want := range tc.want {
				if got.Cents[frame] != want {
					t.Errorf("frame %d: correction = %g, want %g", frame, got.Cents[frame], want)
				}
			}
		})
	}
}
