package prosody

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yh2237/UtauTTS/internal/frontend"
)

func TestFramePitchModelPredictsBoundedContinuousContour(t *testing.T) {
	model := &Model{
		Version: FramePitchModelVersion, FeatureVersion: 1, Mode: "intonation_frame_tcn_accent_bounded",
		FramePitch: &FramePitchModel{
			FeatureNames: []string{"mora_progress"}, InputWeights: [][]float64{{2}}, InputBias: []float64{0},
			OutputWeight: []float64{100}, FrameMS: 10, LowCents: -60, HighCents: 60,
		},
	}
	morae := []frontend.Mora{{Text: "a", Vowel: "a"}, {Pause: true}, {Text: "i", Vowel: "i"}}
	timings := []MoraTiming{{StartMS: 0, DurationMS: 100}, {StartMS: 100, DurationMS: 50}, {StartMS: 150, DurationMS: 100}}
	curve := model.PredictFrameContour(morae, nil, timings, 250, false)
	if curve == nil || curve.FrameMS != 10 || len(curve.Cents) != 26 {
		t.Fatalf("curve=%+v", curve)
	}
	for index, cents := range curve.Cents {
		if cents < -60 || cents > 60 {
			t.Fatalf("frame %d out of bounds: %f", index, cents)
		}
	}
	for index := 10; index < 15; index++ {
		if curve.Cents[index] != 0 {
			t.Fatalf("pause frame %d=%f, want zero", index, curve.Cents[index])
		}
	}
}

func TestFramePitchModelAppliesRendererStrengthAfterCentering(t *testing.T) {
	makeModel := func(strength float64) *Model {
		return &Model{
			Version: FramePitchModelVersion, FeatureVersion: 1, Mode: "intonation_frame_tcn_accent_bounded",
			FramePitch: &FramePitchModel{
				FeatureNames: []string{"frame_position"}, InputWeights: [][]float64{{2}}, InputBias: []float64{0},
				OutputWeight: []float64{100}, FrameMS: 10, LowCents: -1000, HighCents: 1000, RenderStrength: strength,
				RenderSmoothingMS: 0.0001, RenderP99Cents: 10000, RenderMaxCents: 10000,
			},
		}
	}
	morae := []frontend.Mora{{Text: "a", Vowel: "a"}}
	timings := []MoraTiming{{StartMS: 0, DurationMS: 100}}
	full := makeModel(1).PredictFrameContour(morae, nil, timings, 100, false)
	half := makeModel(0.5).PredictFrameContour(morae, nil, timings, 100, false)
	for index := range full.Cents {
		if math.Abs(half.Cents[index]-full.Cents[index]*0.5) > 1e-9 {
			t.Fatalf("frame %d full=%f half=%f", index, full.Cents[index], half.Cents[index])
		}
	}
}

func TestFramePitchModelSafetyLimitsEffectiveContour(t *testing.T) {
	model := &Model{
		Version: FramePitchModelVersion, FeatureVersion: 1, Mode: "intonation_frame_tcn_accent_bounded",
		FramePitch: &FramePitchModel{
			FeatureNames: []string{"frame_position"}, InputWeights: [][]float64{{8}}, InputBias: []float64{-4},
			OutputWeight: []float64{1000}, FrameMS: 10, LowCents: -250, HighCents: 250,
			RenderStrength: 0.32, RenderSmoothingMS: 20, RenderP99Cents: 75, RenderMaxCents: 90,
		},
	}
	curve := model.PredictFrameContour(
		[]frontend.Mora{{Text: "a", Vowel: "a"}}, nil,
		[]MoraTiming{{StartMS: 0, DurationMS: 200}}, 200, false,
	)
	mask := make([]bool, len(curve.Cents))
	for index := range mask {
		mask[index] = true
		if math.Abs(curve.Cents[index]) > 90.000001 {
			t.Fatalf("frame %d exceeded renderer maximum: %f", index, curve.Cents[index])
		}
	}
	if got := absolutePercentile(curve.Cents, mask, 0.99); got > 75.000001 {
		t.Fatalf("p99=%f, want <=75", got)
	}
}

func TestLoadModelRejectsMismatchedHeads(t *testing.T) {
	invalid := []*Model{
		{Version: FramePitchModelVersion, FeatureVersion: 1, Mode: "intonation_frame_tcn_accent_bounded",
			MandarinIntonation: &MandarinIntonationModel{}},
		{Version: 9, FeatureVersion: 1, Mode: "intonation_phrase_anchor_v9"},
	}
	for index, model := range invalid {
		path := filepath.Join(t.TempDir(), fmt.Sprintf("model-%d.json", index))
		if err := model.Save(path); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadModel(path); err == nil {
			t.Fatalf("model %d was accepted", index)
		} else if index == 1 && !strings.Contains(err.Error(), "unsupported prosody model") {
			t.Fatalf("unsupported model error = %v", err)
		}
	}
}
