package prosody

import (
	"fmt"
	"math"
)

func validateMandarinIntonation(model *MandarinIntonationModel) error {
	if model == nil || len(model.Knots) == 0 || len(model.Knots) != len(model.Weights) ||
		len(model.FeatureNames) == 0 || model.Strength <= 0 || model.Strength > 1 ||
		model.MaxCents <= 0 || model.MaxCents > 200 {
		return fmt.Errorf("invalid Mandarin intonation dimensions or limits")
	}
	seen := map[string]bool{}
	previous := 0.0
	for _, name := range model.FeatureNames {
		if name == "" || seen[name] {
			return fmt.Errorf("invalid Mandarin intonation feature %q", name)
		}
		seen[name] = true
	}
	for index, knot := range model.Knots {
		if !finiteMandarin(knot) || knot <= previous || knot >= 1 || len(model.Weights[index]) != len(model.FeatureNames) {
			return fmt.Errorf("invalid Mandarin intonation knot %d", index)
		}
		previous = knot
		for _, weight := range model.Weights[index] {
			if !finiteMandarin(weight) {
				return fmt.Errorf("nonfinite Mandarin intonation weight")
			}
		}
	}
	return nil
}

func finiteMandarin(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

// MandarinCorrectionは各節点の声調補正値を返す。
func (model *MandarinIntonationModel) MandarinCorrection(features map[string]float64) []float64 {
	if validateMandarinIntonation(model) != nil {
		return nil
	}
	result := make([]float64, len(model.Knots))
	for knot, weights := range model.Weights {
		value := 0.0
		for index, name := range model.FeatureNames {
			value += weights[index] * features[name]
		}
		result[knot] = clamp(value, -model.MaxCents, model.MaxCents) * model.Strength
	}
	return result
}
