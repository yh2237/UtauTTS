package connection

import (
	"math"
	"path/filepath"
	"testing"

	"utautts/internal/acoustic"
)

func TestSplitExamplesKeepsGroupsTogether(t *testing.T) {
	examples := []Example{
		{Voicebank: "a", GroupID: "one", Label: 1},
		{Voicebank: "a", GroupID: "one", Label: 0},
		{Voicebank: "b", GroupID: "two", Label: 1},
		{Voicebank: "b", GroupID: "two", Label: 0},
	}
	training, validation := SplitExamples(examples, SplitConfig{ValidationVoicebank: "b"})
	if len(training) != 2 || len(validation) != 2 {
		t.Fatalf("training=%d validation=%d", len(training), len(validation))
	}
	for _, example := range training {
		if example.Voicebank != "a" {
			t.Fatalf("training leaked %q", example.Voicebank)
		}
	}
	for _, example := range validation {
		if example.Voicebank != "b" {
			t.Fatalf("validation leaked %q", example.Voicebank)
		}
	}
}

func TestTrainModelLearnsAcousticDifference(t *testing.T) {
	var training, validation []Example
	for index := 0; index < 40; index++ {
		training = append(training,
			Example{Label: 1, Features: testLearningFeatures(1 + float64(index%3)*0.1)},
			Example{Label: 0, Features: testLearningFeatures(12 + float64(index%3))},
		)
	}
	for index := 0; index < 10; index++ {
		validation = append(validation,
			Example{Label: 1, Features: testLearningFeatures(1.2)},
			Example{Label: 0, Features: testLearningFeatures(13)},
		)
	}
	model, err := TrainModel(training, validation, TrainConfig{Epochs: 300, LearningRate: 0.1, L2: 0.001})
	if err != nil {
		t.Fatal(err)
	}
	if model.Metrics.AUC < 0.99 || model.Metrics.BalancedAccuracy < 0.95 {
		t.Fatalf("metrics=%+v", model.Metrics)
	}
	if model.Predict(testLearningFeatures(1)) <= model.Predict(testLearningFeatures(14)) {
		t.Fatal("smooth boundary did not receive a higher probability")
	}
}

func TestTrainMLPLearnsAcousticDifference(t *testing.T) {
	var training, validation []Example
	for index := 0; index < 40; index++ {
		training = append(training,
			Example{Label: 1, Features: testLearningFeatures(1 + float64(index%3)*0.1)},
			Example{Label: 0, Features: testLearningFeatures(12 + float64(index%3))},
		)
	}
	for index := 0; index < 10; index++ {
		validation = append(validation,
			Example{Label: 1, Features: testLearningFeatures(1.2)},
			Example{Label: 0, Features: testLearningFeatures(13)},
		)
	}
	model, err := TrainModel(training, validation, TrainConfig{
		Epochs: 300, LearningRate: 0.08, L2: 0.001, Model: "mlp", HiddenUnits: 8, Seed: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if model.Mode != "acoustic_join_mlp" || model.Metrics.AUC < 0.99 {
		t.Fatalf("model=%s metrics=%+v", model.Mode, model.Metrics)
	}
	if model.Predict(testLearningFeatures(1)) <= model.Predict(testLearningFeatures(14)) {
		t.Fatal("smooth boundary did not receive a higher probability")
	}
	path := filepath.Join(t.TempDir(), "mlp.json")
	if err := SaveLearnedModel(path, model); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadLearnedModel(path)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(loaded.Predict(testLearningFeatures(1))-model.Predict(testLearningFeatures(1))) > 1e-12 {
		t.Fatal("loaded MLP prediction changed")
	}
}

func TestLoadLearnedModelRejectsZeroScales(t *testing.T) {
	model := &LearnedModel{
		Version: LearnedModelVersion, FeatureVersion: 2, Mode: "acoustic_join_logistic",
		FeatureNames: featureNames(), Means: make([]float64, len(featureNames())),
		Scales: make([]float64, len(featureNames())), Weights: make([]float64, len(featureNames())),
	}
	for index := range model.Scales {
		model.Scales[index] = 1
	}
	model.Scales[3] = 0
	path := filepath.Join(t.TempDir(), "join.json")
	if err := SaveLearnedModel(path, model); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadLearnedModel(path); err == nil {
		t.Fatal("model with a zero scale was accepted")
	}
}

func testLearningFeatures(delta float64) LearningFeatures {
	left, right := make([]float64, 10), make([]float64, 10)
	for index := range right {
		right[index] = delta * (1 + float64(index)/20)
	}
	return LearningFeatures{
		PreviousOutgoing: acoustic.Frame{Valid: true, RMSDB: -20, F0Hz: 200, SpectrumDB: left},
		CurrentIncoming:  acoustic.Frame{Valid: true, RMSDB: -20 + delta, F0Hz: 200 * math.Pow(2, delta/1200), SpectrumDB: right},
		SpectrumDelta:    delta, RMSDelta: delta, F0DeltaCents: delta,
	}
}
