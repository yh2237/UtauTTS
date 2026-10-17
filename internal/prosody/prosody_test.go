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

func TestManualResidualModelAddsMoraCorrectionsWithoutCrossingPause(t *testing.T) {
	model := &Model{
		Version: ManualResidualModelVersion, FeatureVersion: 2, Mode: "intonation_frame_v8_manual_residual",
		BaseModel: &BaseModelReference{ID: "base-intonation", SHA256: strings.Repeat("a", 64)},
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
	model.Mode = "intonation_frame_manual_residual"
	model.BaseModel.ID = "frame-intonation-v9-base"
	path := filepath.Join(t.TempDir(), "manual-residual.json")
	if err := model.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadModel(path)
	if err != nil || loaded.MoraPitchResidual == nil {
		t.Fatalf("manual residual model did not round-trip: model=%#v err=%v", loaded, err)
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
	path := filepath.Join(t.TempDir(), "multitask.json")
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

func TestMoraDurationPhraseFinalLowerBound(t *testing.T) {
	head := &SequencePitchModel{
		FeatureNames: []string{"position"},
		InputWeights: [][]float64{{1}}, InputBias: []float64{0},
		OutputWeight: []float64{-2}, Low: 0.5, High: 2,
	}
	morae := []frontend.Mora{
		{Text: "a", Vowel: "a"}, {Text: "i", Vowel: "i"}, {Text: "、", Pause: true}, {Text: "u", Vowel: "u"}, {Text: "e", Vowel: "e"},
	}
	plain := head.predict(morae, nil)
	if plain[4] >= 1 {
		t.Fatalf("test head should shorten the last mora: %v", plain)
	}
	head.PhraseFinalLow = 1
	floored := head.predict(morae, nil)
	for _, position := range []int{1, 4} {
		if floored[position] < 1 || floored[position] < plain[position] {
			t.Fatalf("phrase-final mora %d was not raised to the bound: %v", position, floored)
		}
	}
	if floored[0] != plain[0] || floored[3] != plain[3] {
		t.Fatalf("non-final morae changed: %v -> %v", plain, floored)
	}
	head.PhraseFinalLow = 0
	head.OutputWeight = []float64{2}
	rising := head.predict(morae, nil)
	head.PhraseStartHigh = 1
	capped := head.predict(morae, nil)
	if rising[3] <= 1 || capped[3] > 1 || capped[0] > 1 || capped[4] != rising[4] {
		t.Fatalf("phrase-start morae were not capped: %v -> %v", rising, capped)
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

func TestLoadModelRejectsMismatchedHeads(t *testing.T) {
	invalid := []*Model{
		{Version: ModelVersion, FeatureVersion: 1, Mode: "speech_prosody_residual",
			FramePitch: &FramePitchModel{FrameMS: 10}},
		{Version: FramePitchModelVersion, FeatureVersion: 1, Mode: "intonation_frame_tcn_accent_bounded",
			MoraDuration: &SequencePitchModel{}},
		{Version: ProsodyMultitaskModelVersion, FeatureVersion: 2, Mode: "prosody_multitask_tcn",
			MoraDuration: &SequencePitchModel{}, FramePitch: &FramePitchModel{FrameMS: 10},
			MandarinIntonation: &MandarinIntonationModel{}},
		{Version: 9, FeatureVersion: 1, Mode: "intonation_phrase_anchor_v9"},
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
