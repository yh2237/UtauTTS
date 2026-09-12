package connection

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"utautts/internal/acoustic"
)

func validFrame() acoustic.Frame {
	return acoustic.Frame{Valid: true, RMSDB: -20, F0Hz: 220, SpectrumDB: []float64{-12, -10}}
}

func TestJoinFeatureVectorUsesStableOrderAndValidity(t *testing.T) {
	features := PairFeatures{
		SpectrumDelta:       2,
		RMSDelta:            3,
		F0DeltaCents:        4,
		VoicingMismatch:     true,
		WaveformCorrelation: 0.7,
		SameSource:          true, ForwardInSource: true, SourceAnchorDistanceMS: 3500,
		CurrentVCV: true,
	}
	values := JoinFeatureVector(features)
	if len(values) != len(joinFeatureNames) || values[0] != 2 || values[3] != 1 || values[7] != 3000 || values[8] != 1 {
		t.Fatalf("feature vector = %#v", values)
	}
	if values[9] != 0 || values[10] != 0 {
		t.Fatalf("invalid frame flags = %#v", values)
	}
}

func TestJoinModelAppliesBoundedCorrectionAndFallsBackWhenUncertain(t *testing.T) {
	model := testJoinModel()
	features := PairFeatures{PreviousOutgoing: validFrame(), CurrentIncoming: validFrame(), SpectrumDelta: 20}
	prediction := model.Predict(features)
	if !prediction.Applied || prediction.Score == prediction.Baseline {
		t.Fatalf("prediction = %#v", prediction)
	}
	if math.Abs(prediction.Score-prediction.Baseline) > model.Blend*model.ScoreScale+1e-9 {
		t.Fatalf("correction exceeded bound: %#v", prediction)
	}
	model.MinConfidence = 1
	fallback := model.Predict(features)
	if fallback.Applied || fallback.Score != fallback.Baseline {
		t.Fatalf("uncertain prediction = %#v", fallback)
	}
}

func TestJoinModelRoundTripsJSON(t *testing.T) {
	model := testJoinModel()
	path := filepath.Join(t.TempDir(), "join.json")
	data, err := json.Marshal(model)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadJoinModel(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != model.ID || len(loaded.Weights) != len(model.Weights) {
		t.Fatalf("loaded model = %#v", loaded)
	}
}

func TestTrainJoinModelUsesOnlyLabeledRows(t *testing.T) {
	rows := make([]JoinAuditRow, 0, 6)
	for index := 0; index < 6; index++ {
		label := float64(index % 2)
		rows = append(rows, JoinAuditRow{
			Features: PairFeatures{SpectrumDelta: float64(index), PreviousOutgoing: validFrame(), CurrentIncoming: validFrame()},
			Label:    &label,
		})
	}
	rows = append(rows, JoinAuditRow{})
	model, report, err := TrainJoinModel(rows, JoinTrainingOptions{Epochs: 50})
	if err != nil {
		t.Fatal(err)
	}
	if report.Examples != 6 || len(model.Weights) != len(joinFeatureNames) {
		t.Fatalf("report=%+v model=%#v", report, model)
	}
}

func testJoinModel() *JoinModel {
	scale := make([]float64, len(joinFeatureNames))
	for index := range scale {
		scale[index] = 1
	}
	return &JoinModel{
		Version: JoinModelVersion, Kind: "logistic_join_ranker", ID: "test",
		FeatureNames: JoinFeatureNames(), Mean: make([]float64, len(joinFeatureNames)),
		Scale: scale, Weights: make([]float64, len(joinFeatureNames)),
		Bias: 1, Blend: 0.35, ScoreScale: 8, MinConfidence: 0,
	}
}
