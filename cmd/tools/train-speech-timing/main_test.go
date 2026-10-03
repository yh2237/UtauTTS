package main

import (
	"encoding/json"
	"math"
	"os"
	"testing"

	"utautts/internal/speechtiming"
)

func TestCheckpointLoadsWithUtauTTS(t *testing.T) {
	path := os.Getenv("SPEECH_TIMING_TEST_CHECKPOINT")
	if path == "" {
		t.Skip("set SPEECH_TIMING_TEST_CHECKPOINT to a trained output")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	model, err := speechtiming.LoadTCN(data)
	if err != nil {
		t.Fatal(err)
	}
	if model.Mels() != 80 || len(model.Phones()) != 40 {
		t.Fatalf("unexpected model: mels=%d phones=%d", model.Mels(), len(model.Phones()))
	}
	fixturePath := os.Getenv("SPEECH_TIMING_TEST_FIXTURE")
	if fixturePath == "" {
		return
	}
	b, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	var fixture parityFixture
	if err = json.Unmarshal(b, &fixture); err != nil {
		t.Fatal(err)
	}
	pred, err := model.Predict(fixture.IDs, fixture.Cont)
	if err != nil {
		t.Fatal(err)
	}
	maxErr := 0.0
	for i := range pred {
		for j := range pred[i] {
			d := math.Abs(float64(pred[i][j] - fixture.Output[i][j]))
			if d > maxErr {
				maxErr = d
			}
		}
	}
	if maxErr > 1e-3 {
		t.Fatalf("loader prediction max error %.6g", maxErr)
	}
}
