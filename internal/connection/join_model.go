package connection

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
)

// JoinModelVersion is the on-disk format version for the optional join model.
const JoinModelVersion = 1

var joinFeatureNames = []string{
	"spectrum_delta_db",
	"rms_delta_db",
	"f0_delta_cents",
	"voicing_mismatch",
	"waveform_correlation",
	"same_source",
	"forward_in_source",
	"source_anchor_distance_ms",
	"current_vcv",
	"previous_valid",
	"current_valid",
}

// JoinFeatureNames returns the stable feature order used by the JSON model.
func JoinFeatureNames() []string {
	return append([]string(nil), joinFeatureNames...)
}

// JoinFeatureVector converts the acoustic measurements into the stable model
// input order. Boolean values are represented as 0 or 1. Missing measurements
// remain zero while the validity features tell the model that they are absent.
func JoinFeatureVector(features PairFeatures) []float64 {
	distance := features.SourceAnchorDistanceMS
	if !isFinite(distance) || distance < 0 {
		distance = 0
	}
	// A very distant same-file jump should not dominate normalization.
	distance = math.Min(distance, 3000)
	return []float64{
		finiteOrZero(features.SpectrumDelta),
		finiteOrZero(features.RMSDelta),
		finiteOrZero(features.F0DeltaCents),
		boolFloat(features.VoicingMismatch),
		finiteOrZero(features.WaveformCorrelation),
		boolFloat(features.SameSource),
		boolFloat(features.ForwardInSource),
		distance,
		boolFloat(features.CurrentVCV),
		boolFloat(features.PreviousOutgoing.Valid),
		boolFloat(features.CurrentIncoming.Valid),
	}
}

// JoinModel is a small logistic ranker exported as JSON. It is intentionally
// evaluated in Go so the optional feature does not add a neural runtime or a
// platform-specific dependency to the application.
type JoinModel struct {
	Version       int       `json:"version"`
	Kind          string    `json:"kind"`
	ID            string    `json:"id"`
	Description   string    `json:"description,omitempty"`
	FeatureNames  []string  `json:"feature_names"`
	Mean          []float64 `json:"mean"`
	Scale         []float64 `json:"scale"`
	Weights       []float64 `json:"weights"`
	Bias          float64   `json:"bias"`
	Blend         float64   `json:"blend"`
	ScoreScale    float64   `json:"score_scale"`
	MinConfidence float64   `json:"min_confidence"`
	Label         string    `json:"label,omitempty"`
	Provenance    string    `json:"provenance,omitempty"`
}

// JoinPrediction contains both the baseline and optional learned decision.
// It is useful to explain an audit result without changing the renderer.
type JoinPrediction struct {
	Baseline    float64
	Probability float64
	Confidence  float64
	Correction  float64
	Score       float64
	Applied     bool
}

// LoadJoinModel reads and validates a model exported by join-ranker.
func LoadJoinModel(path string) (*JoinModel, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	var model JoinModel
	if err := json.Unmarshal(data, &model); err != nil {
		return nil, fmt.Errorf("decode join model %s: %w", path, err)
	}
	if err := model.Validate(); err != nil {
		return nil, fmt.Errorf("validate join model %s: %w", path, err)
	}
	return &model, nil
}

// Validate checks the model contract before it can affect candidate
// selection. An invalid model must never silently alter synthesis.
func (model *JoinModel) Validate() error {
	if model == nil {
		return fmt.Errorf("model is nil")
	}
	if model.Version != JoinModelVersion {
		return fmt.Errorf("unsupported version %d", model.Version)
	}
	if model.Kind != "" && model.Kind != "logistic_join_ranker" {
		return fmt.Errorf("unsupported kind %q", model.Kind)
	}
	if len(model.FeatureNames) != len(joinFeatureNames) {
		return fmt.Errorf("feature count %d, want %d", len(model.FeatureNames), len(joinFeatureNames))
	}
	for index, name := range joinFeatureNames {
		if model.FeatureNames[index] != name {
			return fmt.Errorf("feature %d is %q, want %q", index, model.FeatureNames[index], name)
		}
	}
	if len(model.Mean) != len(joinFeatureNames) || len(model.Scale) != len(joinFeatureNames) || len(model.Weights) != len(joinFeatureNames) {
		return fmt.Errorf("mean, scale, and weights must contain %d values", len(joinFeatureNames))
	}
	for index := range joinFeatureNames {
		if !isFinite(model.Mean[index]) || !isFinite(model.Scale[index]) || !isFinite(model.Weights[index]) || model.Scale[index] <= 0 {
			return fmt.Errorf("invalid normalization or weight at feature %d", index)
		}
	}
	if !isFinite(model.Bias) {
		return fmt.Errorf("bias must be finite")
	}
	if !isFinite(model.Blend) || model.Blend < 0 || model.Blend > 1 {
		return fmt.Errorf("blend must be between 0 and 1")
	}
	if !isFinite(model.ScoreScale) || model.ScoreScale <= 0 {
		return fmt.Errorf("score_scale must be positive")
	}
	if !isFinite(model.MinConfidence) || model.MinConfidence < 0 || model.MinConfidence > 1 {
		return fmt.Errorf("min_confidence must be between 0 and 1")
	}
	return nil
}

// Predict applies the model conservatively. The learned value is a bounded
// correction to HandcraftedScore, so a model cannot replace the existing
// safeguards or create an unbounded path preference.
func (model *JoinModel) Predict(features PairFeatures) JoinPrediction {
	baseline := HandcraftedScore(features)
	result := JoinPrediction{Baseline: baseline, Score: baseline}
	if model == nil || model.Validate() != nil {
		return result
	}
	// A learned ranker cannot recover a missing acoustic frame. Keep the
	// existing fallback for malformed or too-short source material.
	if !features.PreviousOutgoing.Valid || !features.CurrentIncoming.Valid {
		return result
	}
	values := JoinFeatureVector(features)
	logit := model.Bias
	for index, value := range values {
		logit += model.Weights[index] * (value - model.Mean[index]) / model.Scale[index]
	}
	probability := sigmoid(logit)
	confidence := math.Abs(2*probability - 1)
	correction := (2*probability - 1) * model.ScoreScale
	result.Probability = probability
	result.Confidence = confidence
	result.Correction = correction
	if confidence < model.MinConfidence {
		return result
	}
	result.Score = baseline + model.Blend*correction
	result.Applied = model.Blend > 0
	return result
}

// Score evaluates a pair and returns whether the learned correction was used.
func (model *JoinModel) Score(features PairFeatures) (float64, bool) {
	prediction := model.Predict(features)
	return prediction.Score, prediction.Applied
}

func sigmoid(value float64) float64 {
	if value >= 0 {
		exp := math.Exp(-value)
		return 1 / (1 + exp)
	}
	exp := math.Exp(value)
	return exp / (1 + exp)
}

func finiteOrZero(value float64) float64 {
	if !isFinite(value) {
		return 0
	}
	return value
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func boolFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
