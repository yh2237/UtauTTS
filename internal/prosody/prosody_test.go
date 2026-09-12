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

func TestEnglishIntonationModelUsesStressAndPhraseBoundary(t *testing.T) {
	model := &Model{
		ID: "english-intonation-v1", Language: frontend.LanguageEnglish,
		Version: EnglishIntonationModelVersion, FeatureVersion: 1, Mode: "english_intonation_v1",
		EnglishIntonation: &EnglishIntonationModel{
			FrameMS: 10, BaselineStartCents: 42, BaselineEndCents: -42,
			PrimaryStressCents: 64, SecondaryStressCents: 34, UnstressedCents: -14,
			PreStressDipCents: -12, WordDownstepCents: 4, PhraseFinalFallCents: -38,
			QuestionRiseCents: 72, SmoothingMS: 18, LowCents: -180, HighCents: 180,
			P99Cents: 90, MaxCents: 105, PrimaryDurationFactor: 1.18,
			SecondaryDurationFactor: 1.08, UnstressedDurationFactor: 0.88,
			PhraseFinalDurationFactor: 1.08,
		},
	}
	morae := []frontend.Mora{
		{Language: frontend.LanguageEnglish, WordIndex: 0, WordEnd: true, Vowel: "ah", Stress: 0, StressKnown: true},
		{Language: frontend.LanguageEnglish, WordIndex: 1, WordEnd: true, Vowel: "ae", Stress: 1, StressKnown: true},
	}
	timings := []MoraTiming{{StartMS: 0, DurationMS: 120}, {StartMS: 120, DurationMS: 140}}
	statement := model.PredictFrameContour(morae, nil, timings, 260, false)
	question := model.PredictFrameContour(morae, nil, timings, 260, true)
	if statement == nil || question == nil || len(statement.Cents) != 27 {
		t.Fatalf("statement=%#v question=%#v", statement, question)
	}
	if statement.Cents[17] <= statement.Cents[5] {
		t.Fatalf("primary stress did not rise above weak syllable: weak=%.2f stress=%.2f", statement.Cents[5], statement.Cents[17])
	}
	if statement.Cents[26] >= statement.Cents[20] {
		t.Fatalf("statement boundary did not fall: before=%.2f final=%.2f", statement.Cents[20], statement.Cents[26])
	}
	if question.Cents[26] <= statement.Cents[26] {
		t.Fatalf("question boundary did not rise: question=%.2f statement=%.2f", question.Cents[26], statement.Cents[26])
	}
	predictions := model.Predict(morae)
	if predictions[1].DurationFactor <= predictions[0].DurationFactor {
		t.Fatalf("stress duration did not exceed weak duration: %#v", predictions)
	}
	if !model.SupportsLanguage(frontend.LanguageEnglish) || model.SupportsLanguage(frontend.LanguageJapanese) {
		t.Fatalf("language compatibility is wrong")
	}
	if model.RequiresExternalFeatures() || !model.HasFrameContour() {
		t.Fatalf("English model feature requirements are wrong")
	}
	path := filepath.Join(t.TempDir(), "english-intonation-v1.json")
	if err := model.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadModel(path)
	if err != nil || loaded.EnglishIntonation == nil {
		t.Fatalf("English model did not round-trip: model=%#v err=%v", loaded, err)
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
			EnglishIntonation: &EnglishIntonationModel{}},
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
