package connection

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
)

// JoinModelVersionは任意のjoin modelのオンディスク形式バージョン。
const JoinModelVersion = 1

// legacyJoinFeatureNamesはD1以前の11次元モデルが保存した特徴順序。後方互換のため残す。
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

// joinFeatureNamesは現行の特徴順序。D1の新特徴は既存モデルを壊さないよう末尾に追加する。
var joinFeatureNames = append(append([]string(nil), legacyJoinFeatureNames...), "spectral_tilt_delta_db")

// JoinFeatureNamesはJSONモデルが使う固定の特徴量順序を返す。
func JoinFeatureNames() []string {
	return append([]string(nil), joinFeatureNames...)
}

// JoinFeatureVectorは音響測定値をモデル入力の固定順序へ変換する。真偽値は0/1で表し、欠落した測定値は0のままにして有効性フラグで欠落を伝える。
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

// JoinModelはJSONでエクスポートされる小さなロジスティックランカー。ニューラルランタイムやプラットフォーム依存を追加しないため、意図的にGoで評価する。
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

// JoinPredictionはベースラインと任意の学習判定の両方を持つ。レンダラーを変えずにaudit結果を説明するのに役立つ。
type JoinPrediction struct {
	Baseline    float64
	Probability float64
	Confidence  float64
	Correction  float64
	Score       float64
	Applied     bool
}

// LoadJoinModelはjoin-rankerがエクスポートしたモデルを読み込み検証する。
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

// padFeatureSpaceは旧次元のモデルへ欠落した新特徴を重み0・scale 1で補う。既に現行次元なら何もしない。
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

// Validateは候補選択に影響する前にモデル契約を検査する。不正なモデルが合成を黙って変えてはならない。
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

// Predictはモデルを保守的に適用する。学習値はHandcraftedScoreへの有界な補正であり、既存の安全策を置き換えたり、無制限な経路優先を生み出したりしない。
func (model *JoinModel) Predict(features PairFeatures) JoinPrediction {
	baseline := HandcraftedScore(features)
	result := JoinPrediction{Baseline: baseline, Score: baseline}
	if model == nil || model.Validate() != nil {
		return result
	}
	// 学習ランカーは欠落した音響フレームを復元できないため、不正または短すぎる原音は既存のフォールバックを維持する。
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

// Scoreはペアを評価し、学習補正が使われたかを返す。
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
