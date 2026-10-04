package main

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"utautts/internal/frontend"
	"utautts/internal/prosody"
)

func TestTrainedGoExportLoadsAndPredicts(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	path := filepath.Join(root, "out", "frame-intonation-go", "v10-exact-loss.json")
	if _, e := os.Stat(path); e != nil {
		t.Skipf("optional trained export: %v", e)
	}
	model, e := prosody.LoadModel(path)
	if e != nil {
		t.Fatal(e)
	}
	rows, e := loadRecords(filepath.Join(root, "out", "mfa-align-20261002", "base-mfa.jsonl"))
	if e != nil {
		t.Fatal(e)
	}
	r := rows[0]
	morae := make([]frontend.Mora, len(r.Tokens))
	timings := make([]prosody.MoraTiming, len(r.Tokens))
	for i, token := range r.Tokens {
		morae[i] = frontend.Mora{Text: token.Mora, Vowel: token.Vowel, Pause: token.Pause}
		timings[i] = prosody.MoraTiming{StartMS: token.Start, DurationMS: token.End - token.Start}
	}
	curve := model.PredictFrameContour(morae, nil, timings, r.Tokens[len(r.Tokens)-1].End, false)
	if curve == nil || len(curve.Cents) == 0 {
		t.Fatal("no predicted contour")
	}
	for i, v := range curve.Cents {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < -250.00001 || v > 250.00001 {
			t.Fatalf("contour frame %d = %.6f", i, v)
		}
	}
	t.Logf("loaded trained Go export; predicted %d contour frames", len(curve.Cents))
}
