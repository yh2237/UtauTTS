package prosody

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"utautts/internal/frontend"
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

func TestManualResidualModelAddsMoraCorrectionsWithoutCrossingPause(t *testing.T) {
	model := &Model{
		Version: ManualResidualModelVersion, FeatureVersion: 2, Mode: "intonation_frame_v8_manual_residual",
		BaseModel: &BaseModelReference{ID: "frame-intonation-v8", SHA256: strings.Repeat("a", 64)},
		FramePitch: &FramePitchModel{
			FeatureNames: []string{"bias"}, InputWeights: [][]float64{{0}}, InputBias: []float64{0},
			OutputWeight: []float64{0}, FrameMS: 10, LowCents: -250, HighCents: 250,
		},
		MoraPitchResidual: &MoraPitchResidualModel{
			FeatureNames: []string{"bias"}, InputWeights: [][]float64{{0}}, InputBias: []float64{0},
			OutputWeight: []float64{0}, OutputBias: 30,
		},
		ResidualLimits: &ResidualLimits{LowCents: -120, HighCents: 120, SmoothingMS: 20},
	}
	morae := []frontend.Mora{{Text: "あ", Vowel: "a"}, {Pause: true}, {Text: "い", Vowel: "i"}}
	timings := []MoraTiming{{StartMS: 0, DurationMS: 100}, {StartMS: 100, DurationMS: 50}, {StartMS: 150, DurationMS: 100}}
	curve := model.PredictFrameContour(morae, nil, timings, 250, false)
	if curve == nil {
		t.Fatal("manual residual model returned no contour")
	}
	if curve.Cents[5] < 29.9 || curve.Cents[20] < 29.9 {
		t.Fatalf("residual was not added at mora centers: %.2f %.2f", curve.Cents[5], curve.Cents[20])
	}
	for index := 10; index < 15; index++ {
		if curve.Cents[index] != 0 {
			t.Fatalf("residual crossed pause at frame %d: %.2f", index, curve.Cents[index])
		}
	}
	path := filepath.Join(t.TempDir(), "manual-residual.json")
	if err := model.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadModel(path)
	if err != nil || loaded.MoraPitchResidual == nil {
		t.Fatalf("manual residual model did not round-trip: model=%#v err=%v", loaded, err)
	}
}

func TestSequencePitchUsesTemporalContext(t *testing.T) {
	model := &Model{
		Version: SequenceModelVersion, FeatureVersion: 1, Mode: "intonation_tcn",
		SequencePitch: &SequencePitchModel{
			FeatureNames: []string{"phrase_start"},
			InputWeights: [][]float64{{1}}, InputBias: []float64{0},
			Layers: []SequencePitchLayer{{
				Dilation: 1, Weights: [][][]float64{{{0.5, 0, 0}}}, Bias: []float64{0},
			}},
			OutputWeight: []float64{1}, Low: 0.9, High: 1.1,
		},
	}
	morae := []frontend.Mora{{Text: "a", Vowel: "a"}, {Text: "i", Vowel: "i"}, {Text: "u", Vowel: "u"}}
	predicted := model.Predict(morae)
	if predicted[0].PitchFactor <= predicted[1].PitchFactor {
		t.Fatalf("sequence pitch did not use the start feature: %#v", predicted)
	}
	if predicted[1].PitchFactor <= predicted[2].PitchFactor {
		t.Fatalf("temporal convolution did not carry context forward: %#v", predicted)
	}
	path := filepath.Join(t.TempDir(), "sequence.json")
	if err := model.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadModel(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Predict(morae); math.Abs(got[1].PitchFactor-predicted[1].PitchFactor) > 1e-9 {
		t.Fatalf("sequence model round trip changed prediction: %#v != %#v", got, predicted)
	}
}

func TestProsodyMultitaskModelPredictsMoraDurationAndLoads(t *testing.T) {
	model := &Model{
		Version: ProsodyMultitaskModelVersion, FeatureVersion: 2, Mode: "prosody_multitask_tcn",
		MoraDuration: &SequencePitchModel{
			FeatureNames: []string{"position"},
			InputWeights: [][]float64{{1}}, InputBias: []float64{0},
			OutputWeight: []float64{2}, Low: 0.5, High: 2,
		},
		FramePitch: &FramePitchModel{
			FeatureNames: []string{"frame_position"}, InputWeights: [][]float64{{1}}, InputBias: []float64{0},
			OutputWeight: []float64{1}, FrameMS: 10, LowCents: -120, HighCents: 120,
		},
	}
	morae := []frontend.Mora{
		{Text: "a", Vowel: "a"}, {Text: "i", Vowel: "i"}, {Text: "u", Vowel: "u"},
	}
	predicted := model.Predict(morae)
	if predicted[0].DurationFactor >= 1 || predicted[2].DurationFactor <= 1 {
		t.Fatalf("mora duration head did not produce relative factors: %#v", predicted)
	}
	if !model.HasFrameContour() {
		t.Fatal("multitask model did not report frame contour")
	}
	path := filepath.Join(t.TempDir(), "prosody-multitask-v1.json")
	if err := model.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadModel(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Predict(morae)[2].DurationFactor; math.Abs(got-predicted[2].DurationFactor) > 1e-9 {
		t.Fatalf("multitask round trip changed duration prediction: %v != %v", got, predicted[2].DurationFactor)
	}
}

func TestProsodyMultitaskModelReportsExternalFeaturesFromEitherHead(t *testing.T) {
	model := &Model{
		Version: ProsodyMultitaskModelVersion, FeatureVersion: 2, Mode: "prosody_multitask_tcn",
		MoraDuration: &SequencePitchModel{
			FeatureNames: []string{"accent_high"}, InputWeights: [][]float64{{1}}, InputBias: []float64{0},
			OutputWeight: []float64{1}, Low: 0.5, High: 2,
		},
		FramePitch: &FramePitchModel{
			FeatureNames: []string{"frame_position"}, InputWeights: [][]float64{{1}}, InputBias: []float64{0},
			OutputWeight: []float64{1}, FrameMS: 10, LowCents: -120, HighCents: 120,
		},
	}
	if !model.RequiresExternalFeatures() {
		t.Fatal("mora duration accent features were not reported")
	}
}

func TestLoadAccentSequenceModelVersions(t *testing.T) {
	for _, test := range []struct {
		version int
		mode    string
	}{
		{AccentSequenceModelVersion, "intonation_tcn_accent"},
		{BoundedSequenceModelVersion, "intonation_tcn_accent_bounded"},
	} {
		t.Run(test.mode, func(t *testing.T) {
			model := &Model{
				Version: test.version, FeatureVersion: 1, Mode: test.mode,
				SequencePitch: &SequencePitchModel{
					FeatureNames: []string{"bias"},
					InputWeights: [][]float64{{1}}, InputBias: []float64{0},
					OutputWeight: []float64{1}, Low: 0.97, High: 1.03,
				},
			}
			path := filepath.Join(t.TempDir(), "model.json")
			if err := model.Save(path); err != nil {
				t.Fatal(err)
			}
			loaded, err := LoadModel(path)
			if err != nil {
				t.Fatal(err)
			}
			if loaded.Version != test.version || loaded.Mode != test.mode {
				t.Fatalf("loaded model = %d/%q", loaded.Version, loaded.Mode)
			}
		})
	}
}

func TestAccentSequenceModelUsesExternalFeatureFrames(t *testing.T) {
	model := &Model{
		Version: AccentSequenceModelVersion, FeatureVersion: 1, Mode: "intonation_tcn_accent",
		SequencePitch: &SequencePitchModel{
			FeatureNames: []string{"accent_high"},
			InputWeights: [][]float64{{2}}, InputBias: []float64{0},
			OutputWeight: []float64{1}, Low: 0.8, High: 1.2,
		},
	}
	if !model.RequiresExternalFeatures() {
		t.Fatal("accent model did not report its external feature requirement")
	}
	morae := []frontend.Mora{{Text: "あ", Vowel: "a"}, {Text: "い", Vowel: "i"}}
	predicted := model.PredictWithFeatures(morae, []FeatureFrame{{"accent_high": 1}, {"accent_high": 0}})
	if predicted[0].PitchFactor <= predicted[1].PitchFactor {
		t.Fatalf("external accent feature did not affect prediction: %#v", predicted)
	}
}

func TestStandardAccentContourFollowsHighLowPattern(t *testing.T) {
	model := &Model{
		Version: StandardAccentModelVersion, FeatureVersion: 1, Mode: "standard_japanese_accent",
		StandardAccent: &StandardAccentModel{
			FrameMS: 10, AccentRangeCents: 70, DeclinationCents: 10,
			QuestionRiseCents: 35, SmoothingMS: 20, P99Cents: 65, MaxCents: 80,
		},
	}
	if !model.RequiresExternalFeatures() || !model.HasFrameContour() {
		t.Fatal("standard accent model did not report its feature/contour requirements")
	}
	morae := []frontend.Mora{{Text: "あ", Vowel: "a"}, {Text: "い", Vowel: "i"}, {Text: "う", Vowel: "u"}}
	frames := []FeatureFrame{
		{"accent_high": 0, "accent_position": 0.33},
		{"accent_high": 1, "accent_position": 0.66},
		{"accent_high": 0, "accent_position": 1},
	}
	timings := []MoraTiming{{StartMS: 0, DurationMS: 100}, {StartMS: 100, DurationMS: 100}, {StartMS: 200, DurationMS: 100}}
	contour := model.PredictFrameContour(morae, frames, timings, 300, false)
	if contour == nil {
		t.Fatal("standard accent model returned no contour")
	}
	if contour.Cents[15] <= contour.Cents[5] || contour.Cents[25] >= contour.Cents[15] {
		t.Fatalf("contour did not follow low-high-low accent: %.2f %.2f %.2f", contour.Cents[5], contour.Cents[15], contour.Cents[25])
	}
	path := filepath.Join(t.TempDir(), "standard-accent.json")
	if err := model.Save(path); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadModel(path); err != nil {
		t.Fatal(err)
	}
}

func TestPhraseAnchorV9ProducesSmoothContourAndLoads(t *testing.T) {
	model := &Model{
		Version: StandardAccentModelVersion, FeatureVersion: 1, Mode: "intonation_phrase_anchor_v9",
		PhrasePitch: &PhrasePitchModel{
			FeatureNames: []string{"bias"},
			Weights:      [][]float64{{0}, {0}, {0}, {0}}, Bias: []float64{0, 20, -10, 30},
			FrameMS: 10, LowCents: -120, HighCents: 120,
			AccentRangeCents: 60, DeclinationCents: 10, SmoothingMS: 20,
			P99Cents: 90, MaxCents: 100,
		},
	}
	morae := []frontend.Mora{{Text: "a", Vowel: "a"}, {Text: "i", Vowel: "i"}, {Text: "u", Vowel: "u"}}
	frames := []FeatureFrame{
		{"accent_phrase_start": 1, "accent_phrase_position": 1, "accent_position": 0.33, "accent_nucleus_position": 0.66, "accent_high": 0},
		{"accent_position": 0.66, "accent_nucleus_position": 0.66, "accent_high": 1},
		{"accent_phrase_end": 1, "accent_position": 1, "accent_nucleus_position": 0.66, "accent_high": 0},
	}
	timings := []MoraTiming{{StartMS: 0, DurationMS: 100}, {StartMS: 100, DurationMS: 100}, {StartMS: 200, DurationMS: 100}}
	curve := model.PredictFrameContour(morae, frames, timings, 300, false)
	if curve == nil || len(curve.Cents) < 3 {
		t.Fatal("v9 phrase model returned no contour")
	}
	for _, value := range curve.Cents {
		if value < -120 || value > 120 {
			t.Fatalf("v9 contour exceeded bounds: %f", value)
		}
	}
	if curve.Cents[0] == curve.Cents[len(curve.Cents)-1] {
		t.Fatalf("v9 anchors did not affect contour: %v", curve.Cents)
	}
	path := filepath.Join(t.TempDir(), "v9.json")
	if err := model.Save(path); err != nil {
		t.Fatal(err)
	}
	if loaded, err := LoadModel(path); err != nil || loaded.PhrasePitch == nil {
		t.Fatalf("v9 model did not round-trip: model=%#v err=%v", loaded, err)
	}
	model.Mode = "intonation_phrase_anchor_v9_1"
	v91Path := filepath.Join(t.TempDir(), "v9-1.json")
	if err := model.Save(v91Path); err != nil {
		t.Fatal(err)
	}
	if loaded, err := LoadModel(v91Path); err != nil || loaded.PhrasePitch == nil {
		t.Fatalf("v9.1 model did not round-trip: model=%#v err=%v", loaded, err)
	}
}

func TestLoadModelRejectsMismatchedHeads(t *testing.T) {
	invalid := []*Model{
		{Version: ModelVersion, FeatureVersion: 1, Mode: "speech_prosody_residual",
			FramePitch: &FramePitchModel{FrameMS: 10}},
		{Version: FramePitchModelVersion, FeatureVersion: 1, Mode: "intonation_frame_tcn_accent_bounded",
			SequencePitch: &SequencePitchModel{}},
		{Version: ProsodyMultitaskModelVersion, FeatureVersion: 2, Mode: "prosody_multitask_tcn",
			MoraDuration: &SequencePitchModel{}, FramePitch: &FramePitchModel{FrameMS: 10},
			PhrasePitch: &PhrasePitchModel{}},
	}
	for index, model := range invalid {
		path := filepath.Join(t.TempDir(), fmt.Sprintf("model-%d.json", index))
		if err := model.Save(path); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadModel(path); err == nil {
			t.Fatalf("model %d with mismatched heads was accepted", index)
		}
	}
}
