package connection

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
)

const JoinModelVersion = 1

// 旧11次元モデルの読込用。特徴順序を変えない。
var legacyJoinFeatureNames = []string{
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

// 既存モデルの特徴順序を保つため、新特徴は末尾へ追加する。
var joinFeatureNames = append(append([]string(nil), legacyJoinFeatureNames...), "spectral_tilt_delta_db")

func JoinFeatureNames() []string {
	return append([]string(nil), joinFeatureNames...)
}

// 特徴順序を固定し、欠測は値0と有効性フラグで伝える。
func JoinFeatureVector(features PairFeatures) []float64 {
	distance := features.SourceAnchorDistanceMS
	if !isFinite(distance) || distance < 0 {
		distance = 0
	}
	// 同一ファイル内の遠すぎるジャンプが正規化を支配しないようにする。
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
		finiteOrZero(features.SpectralTiltDelta),
	}
}

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

type JoinPrediction struct {
	Baseline    float64
	Probability float64
	Confidence  float64
	Correction  float64
	Score       float64
	Applied     bool
}

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
	// 旧11次元モデルは新特徴の重みを0として読み込む。
	model.padFeatureSpace()
	return &model, nil
}

// 旧モデルの予測を変えないよう、新特徴を重み0・尺度1で補う。
func (model *JoinModel) padFeatureSpace() {
	missing := len(joinFeatureNames) - len(model.FeatureNames)
	if missing <= 0 {
		return
	}
	model.FeatureNames = JoinFeatureNames()
	model.Mean = append(model.Mean, make([]float64, missing)...)
	for index := 0; index < missing; index++ {
		model.Scale = append(model.Scale, 1)
	}
	model.Weights = append(model.Weights, make([]float64, missing)...)
}

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
	expected := joinFeatureNames
	if len(model.FeatureNames) == len(legacyJoinFeatureNames) {
		// 旧11次元モデルも受理し、読み込み時に新特徴を0重みで補う。
		expected = legacyJoinFeatureNames
	} else if len(model.FeatureNames) != len(joinFeatureNames) {
		return fmt.Errorf("feature count %d, want %d", len(model.FeatureNames), len(joinFeatureNames))
	}
	for index, name := range expected {
		if model.FeatureNames[index] != name {
			return fmt.Errorf("feature %d is %q, want %q", index, model.FeatureNames[index], name)
		}
	}
	count := len(expected)
	if len(model.Mean) != count || len(model.Scale) != count || len(model.Weights) != count {
		return fmt.Errorf("mean, scale, and weights must contain %d values", count)
	}
	for index := range expected {
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

// 学習値は規則スコアへの有界な補正に留め、既存の安全策を保つ。
func (model *JoinModel) Predict(features PairFeatures) JoinPrediction {
	baseline := HandcraftedScore(features)
	result := JoinPrediction{Baseline: baseline, Score: baseline}
	if model == nil || model.Validate() != nil {
		return result
	}
	// 不正・短すぎる原音は、学習値ではなく既存の代替スコアを使う。
	if !features.PreviousOutgoing.Valid || !features.CurrentIncoming.Valid {
		return result
	}
	values := JoinFeatureVector(features)
	logit := model.Bias
	for index, value := range values {
		if index >= len(model.Weights) {
			// 未補正の旧次元モデルは新特徴を無視する。
			break
		}
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
