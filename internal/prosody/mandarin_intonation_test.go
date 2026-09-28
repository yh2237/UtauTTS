package prosody

import (
	"math"
	"path/filepath"
	"testing"
)

func TestMandarinIntonationModelLoadsAndBoundsCorrection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mandarin.json")
	model := &Model{
		Language: "zh", Version: MandarinIntonationModelVersion, FeatureVersion: 1,
		Mode: "mandarin_intonation_v1",
		MandarinIntonation: &MandarinIntonationModel{
			FeatureNames: []string{"bias", "tone_1"}, Knots: []float64{0.25, 0.75},
			Weights: [][]float64{{10, 200}, {-10, -200}}, Strength: 0.5, MaxCents: 100,
		},
	}
	if err := model.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadModel(path)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.SupportsLanguage("zh") || loaded.SupportsLanguage("ja") {
		t.Fatal("Mandarin model language selection failed")
	}
	values := loaded.MandarinIntonation.MandarinCorrection(map[string]float64{"bias": 1, "tone_1": 1})
	if len(values) != 2 || values[0] != 50 || values[1] != -50 {
		t.Fatalf("bounded correction = %v", values)
	}
	model.MandarinIntonation.Weights[0][0] = math.NaN()
	if validateMandarinIntonation(model.MandarinIntonation) == nil {
		t.Fatal("nonfinite correction weight accepted")
	}
}
