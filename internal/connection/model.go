package connection

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
)

const LearnedModelVersion = 3

type Example struct {
	Voicebank string
	GroupID   string
	Label     int
	Features  LearningFeatures
	Weight    float64
}

type LearnedModel struct {
	Version        int          `json:"version"`
	FeatureVersion int          `json:"feature_version"`
	Mode           string       `json:"mode"`
	FeatureNames   []string     `json:"feature_names"`
	Means          []float64    `json:"means"`
	Scales         []float64    `json:"scales"`
	Weights        []float64    `json:"weights"`
	Bias           float64      `json:"bias"`
	HiddenWeights  [][]float64  `json:"hidden_weights,omitempty"`
	HiddenBias     []float64    `json:"hidden_bias,omitempty"`
	OutputWeights  []float64    `json:"output_weights,omitempty"`
	ScoreScale     float64      `json:"score_scale,omitempty"`
	Training       TrainingInfo `json:"training"`
	Metrics        Metrics      `json:"validation_metrics"`
}

type TrainingInfo struct {
	Records      int      `json:"records"`
	Positive     int      `json:"positive"`
	Negative     int      `json:"negative"`
	Voicebanks   []string `json:"voicebanks"`
	Epochs       int      `json:"epochs"`
	LearningRate float64  `json:"learning_rate"`
	L2           float64  `json:"l2"`
	Model        string   `json:"model,omitempty"`
	HiddenUnits  int      `json:"hidden_units,omitempty"`
	Seed         int64    `json:"seed,omitempty"`
}

type Metrics struct {
	Records          int     `json:"records"`
	Positive         int     `json:"positive"`
	Negative         int     `json:"negative"`
	Accuracy         float64 `json:"accuracy"`
	BalancedAccuracy float64 `json:"balanced_accuracy"`
	LogLoss          float64 `json:"log_loss"`
	AUC              float64 `json:"auc"`
}

type TrainConfig struct {
	Epochs       int
	LearningRate float64
	L2           float64
	Model        string
	HiddenUnits  int
	Seed         int64
}

type SplitConfig struct {
	ValidationVoicebank string
	ValidationFraction  float64
	Seed                uint64
}

type scoredLabel struct {
	probability float64
	label       int
}

func SplitExamples(examples []Example, config SplitConfig) (training, validation []Example) {
	for _, example := range examples {
		isValidation := example.Voicebank == config.ValidationVoicebank && config.ValidationVoicebank != ""
		if config.ValidationVoicebank == "" && config.ValidationFraction > 0 {
			isValidation = groupFraction(example.Voicebank, example.GroupID, config.Seed) < config.ValidationFraction
		}
		if isValidation {
			validation = append(validation, example)
		} else {
			training = append(training, example)
		}
	}
	return training, validation
}

func TrainModel(training, validation []Example, config TrainConfig) (*LearnedModel, error) {
	if config.Epochs <= 0 || config.LearningRate <= 0 || config.L2 < 0 {
		return nil, errors.New("invalid training configuration")
	}
	if config.Model == "" {
		config.Model = "logistic"
	}
	if config.Model != "logistic" && config.Model != "mlp" {
		return nil, fmt.Errorf("unknown model type %q", config.Model)
	}
	if config.HiddenUnits <= 0 {
		config.HiddenUnits = 32
	}
	if config.Seed == 0 {
		config.Seed = 1
	}
	training = validExamples(training)
	validation = validExamples(validation)
	positive, negative := labelCounts(training)
	if positive == 0 || negative == 0 {
		return nil, fmt.Errorf("training data needs both labels: positive=%d negative=%d", positive, negative)
	}
	names := featureNames()
	vectors := make([][]float64, len(training))
	means := make([]float64, len(names))
	for index, example := range training {
		vectors[index] = featureVector(example.Features)
		for feature, value := range vectors[index] {
			means[feature] += value
		}
	}
	for feature := range means {
		means[feature] /= float64(len(vectors))
	}
	scales := make([]float64, len(names))
	for _, vector := range vectors {
		for feature, value := range vector {
			delta := value - means[feature]
			scales[feature] += delta * delta
		}
	}
	for feature := range scales {
		scales[feature] = math.Sqrt(scales[feature] / float64(len(vectors)))
		if scales[feature] < 1e-8 {
			scales[feature] = 1
		}
	}
	for _, vector := range vectors {
		normalize(vector, means, scales)
	}

	weights := make([]float64, len(names))
	bias := 0.0
	positiveWeight := float64(len(training)) / (2 * float64(positive))
	negativeWeight := float64(len(training)) / (2 * float64(negative))
	if config.Model == "logistic" {
		for epoch := 0; epoch < config.Epochs; epoch++ {
			gradient := make([]float64, len(weights))
			biasGradient := 0.0
			for index, example := range training {
				probability := sigmoid(bias + dotVector(weights, vectors[index]))
				classWeight := negativeWeight
				if example.Label == 1 {
					classWeight = positiveWeight
				}
				exampleWeight := example.Weight
				if exampleWeight <= 0 {
					exampleWeight = 1
				}
				delta := exampleWeight * classWeight * (probability - float64(example.Label))
				biasGradient += delta
				for feature, value := range vectors[index] {
					gradient[feature] += delta * value
				}
			}
			rate := config.LearningRate / math.Sqrt(1+float64(epoch)*0.02)
			bias -= rate * biasGradient / float64(len(training))
			for feature := range weights {
				gradient[feature] = gradient[feature]/float64(len(training)) + config.L2*weights[feature]
				weights[feature] -= rate * gradient[feature]
			}
		}
	}
	mode := "acoustic_join_logistic"
	recordedHiddenUnits := 0
	if config.Model == "mlp" {
		recordedHiddenUnits = config.HiddenUnits
	}
	model := &LearnedModel{
		Version: LearnedModelVersion, FeatureVersion: 2, Mode: mode,
		FeatureNames: names, Means: means, Scales: scales, Weights: weights, Bias: bias,
		ScoreScale: 4,
		Training: TrainingInfo{Records: len(training), Positive: positive, Negative: negative,
			Voicebanks: voicebanks(training), Epochs: config.Epochs,
			LearningRate: config.LearningRate, L2: config.L2, Model: config.Model,
			HiddenUnits: recordedHiddenUnits, Seed: config.Seed},
	}
	if config.Model == "mlp" {
		model.Mode = "acoustic_join_mlp"
		model.Weights = nil
		model.Bias = 0
		model.HiddenWeights, model.HiddenBias, model.OutputWeights, model.Bias = trainMLP(
			training, vectors, len(names), config, positiveWeight, negativeWeight,
		)
	}
	model.Metrics = EvaluateModel(model, validation)
	return model, nil
}

func (model *LearnedModel) Predict(features LearningFeatures) float64 {
	vector := featureVector(features)
	if len(model.Means) < len(vector) {
		vector = vector[:len(model.Means)]
	}
	normalize(vector, model.Means, model.Scales)
	if model.Mode == "acoustic_join_mlp" {
		hidden := make([]float64, len(model.HiddenWeights))
		for unit := range hidden {
			hidden[unit] = math.Tanh(model.HiddenBias[unit] + dotVector(model.HiddenWeights[unit], vector))
		}
		return sigmoid(model.Bias + dotVector(model.OutputWeights, hidden))
	}
	return sigmoid(model.Bias + dotVector(model.Weights, vector))
}

// LearnedScoreはクリップしたlogitを接続ボーナスと同じ尺度へ変換する。
func LearnedScore(features PairFeatures, model *LearnedModel) (score, probability float64) {
	score = sourceContinuityScore(features)
	learning := ToLearningFeatures(features)
	// 既存モデルの入力次元を変えずVCVの想定内の無声閉鎖を強い負例にしない。
	if features.CurrentVCV && features.VoicingMismatch {
		learning.VoicingMismatch = false
	}
	if model == nil || !learning.Valid() {
		return score, 0.5
	}
	probability = model.Predict(learning)
	boundedProbability := math.Max(1e-9, math.Min(1-1e-9, probability))
	logit := math.Log(boundedProbability / (1 - boundedProbability))
	logit = math.Max(-2, math.Min(2, logit))
	scale := model.ScoreScale
	if scale <= 0 {
		scale = 4
	}
	return score + logit*scale, probability
}

func EvaluateModel(model *LearnedModel, examples []Example) Metrics {
	examples = validExamples(examples)
	metrics := Metrics{Records: len(examples)}
	scores := make([]scoredLabel, 0, len(examples))
	var correct, truePositive, trueNegative int
	for _, example := range examples {
		probability := model.Predict(example.Features)
		if example.Label == 1 {
			metrics.Positive++
		} else {
			metrics.Negative++
		}
		predicted := 0
		if probability >= 0.5 {
			predicted = 1
		}
		if predicted == example.Label {
			correct++
		}
		if predicted == 1 && example.Label == 1 {
			truePositive++
		}
		if predicted == 0 && example.Label == 0 {
			trueNegative++
		}
		p := math.Max(1e-9, math.Min(1-1e-9, probability))
		metrics.LogLoss -= float64(example.Label)*math.Log(p) + float64(1-example.Label)*math.Log(1-p)
		scores = append(scores, scoredLabel{probability, example.Label})
	}
	if metrics.Records > 0 {
		metrics.Accuracy = float64(correct) / float64(metrics.Records)
		metrics.LogLoss /= float64(metrics.Records)
	}
	if metrics.Positive > 0 && metrics.Negative > 0 {
		metrics.BalancedAccuracy = (float64(truePositive)/float64(metrics.Positive) + float64(trueNegative)/float64(metrics.Negative)) / 2
		metrics.AUC = auc(scores)
	}
	return metrics
}

func SaveLearnedModel(path string, model *LearnedModel) error {
	data, err := json.MarshalIndent(model, "", "  ")
	if err != nil {
		return err
	}
	if directory := filepath.Dir(path); directory != "." {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func LoadLearnedModel(path string) (*LearnedModel, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var model LearnedModel
	if err := json.Unmarshal(data, &model); err != nil {
		return nil, err
	}
	current := model.Version == LearnedModelVersion && model.FeatureVersion == 2
	if !current || (model.Mode != "acoustic_join_logistic" && model.Mode != "acoustic_join_mlp") {
		return nil, fmt.Errorf("unsupported join model version %d/%d", model.Version, model.FeatureVersion)
	}
	length := len(featureNames())
	if len(model.Means) != length || len(model.Scales) != length {
		return nil, errors.New("invalid join model feature dimensions")
	}
	if !allFinite(model.Means) {
		return nil, errors.New("join model means contain non-finite values")
	}
	for index, scale := range model.Scales {
		if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
			return nil, fmt.Errorf("join model scale %d is not positive and finite", index)
		}
	}
	if model.Mode == "acoustic_join_logistic" {
		if len(model.Weights) != length {
			return nil, errors.New("invalid logistic model dimensions")
		}
		if !allFinite(model.Weights) || !finiteValue(model.Bias) {
			return nil, errors.New("logistic model weights contain non-finite values")
		}
	}
	if model.Mode == "acoustic_join_mlp" {
		if !validMLPDimensions(&model, length) {
			return nil, errors.New("invalid MLP model dimensions")
		}
		if !finiteMLP(&model) {
			return nil, errors.New("MLP model weights contain non-finite values")
		}
	}
	if model.ScoreScale <= 0 {
		model.ScoreScale = 4
	}
	return &model, nil
}

func allFinite(values []float64) bool {
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return true
}

func finiteValue(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func finiteMLP(model *LearnedModel) bool {
	if !finiteValue(model.Bias) || !allFinite(model.HiddenBias) || !allFinite(model.OutputWeights) {
		return false
	}
	for _, weights := range model.HiddenWeights {
		if !allFinite(weights) {
			return false
		}
	}
	return true
}

func trainMLP(examples []Example, vectors [][]float64, featureCount int, config TrainConfig, positiveWeight, negativeWeight float64) ([][]float64, []float64, []float64, float64) {
	rng := rand.New(rand.NewSource(config.Seed))
	hiddenWeights := make([][]float64, config.HiddenUnits)
	hiddenBias := make([]float64, config.HiddenUnits)
	outputWeights := make([]float64, config.HiddenUnits)
	inputScale := math.Sqrt(2 / float64(featureCount+config.HiddenUnits))
	outputScale := math.Sqrt(2 / float64(config.HiddenUnits+1))
	for unit := range hiddenWeights {
		hiddenWeights[unit] = make([]float64, featureCount)
		for feature := range hiddenWeights[unit] {
			hiddenWeights[unit][feature] = rng.NormFloat64() * inputScale
		}
		outputWeights[unit] = rng.NormFloat64() * outputScale
	}
	outputBias := 0.0
	for epoch := 0; epoch < config.Epochs; epoch++ {
		hiddenGradient := make([][]float64, config.HiddenUnits)
		hiddenBiasGradient := make([]float64, config.HiddenUnits)
		outputGradient := make([]float64, config.HiddenUnits)
		for unit := range hiddenGradient {
			hiddenGradient[unit] = make([]float64, featureCount)
		}
		outputBiasGradient := 0.0
		for index, example := range examples {
			hidden := make([]float64, config.HiddenUnits)
			for unit := range hidden {
				hidden[unit] = math.Tanh(hiddenBias[unit] + dotVector(hiddenWeights[unit], vectors[index]))
			}
			probability := sigmoid(outputBias + dotVector(outputWeights, hidden))
			classWeight := negativeWeight
			if example.Label == 1 {
				classWeight = positiveWeight
			}
			exampleWeight := example.Weight
			if exampleWeight <= 0 {
				exampleWeight = 1
			}
			delta := exampleWeight * classWeight * (probability - float64(example.Label))
			outputBiasGradient += delta
			for unit := range hidden {
				outputGradient[unit] += delta * hidden[unit]
				hiddenDelta := delta * outputWeights[unit] * (1 - hidden[unit]*hidden[unit])
				hiddenBiasGradient[unit] += hiddenDelta
				for feature, value := range vectors[index] {
					hiddenGradient[unit][feature] += hiddenDelta * value
				}
			}
		}
		rate := config.LearningRate / math.Sqrt(1+float64(epoch)*0.02)
		n := float64(len(examples))
		outputBias -= rate * outputBiasGradient / n
		for unit := range hiddenWeights {
			outputWeights[unit] -= rate * (outputGradient[unit]/n + config.L2*outputWeights[unit])
			hiddenBias[unit] -= rate * hiddenBiasGradient[unit] / n
			for feature := range hiddenWeights[unit] {
				hiddenWeights[unit][feature] -= rate * (hiddenGradient[unit][feature]/n + config.L2*hiddenWeights[unit][feature])
			}
		}
	}
	return hiddenWeights, hiddenBias, outputWeights, outputBias
}

func validMLPDimensions(model *LearnedModel, features int) bool {
	if len(model.HiddenWeights) == 0 || len(model.HiddenBias) != len(model.HiddenWeights) || len(model.OutputWeights) != len(model.HiddenWeights) {
		return false
	}
	for _, weights := range model.HiddenWeights {
		if len(weights) != features {
			return false
		}
	}
	return true
}

func validExamples(examples []Example) []Example {
	result := make([]Example, 0, len(examples))
	for _, example := range examples {
		if example.Features.Valid() && (example.Label == 0 || example.Label == 1) {
			result = append(result, example)
		}
	}
	return result
}

func featureNames() []string {
	result := []string{"spectrum_delta_db", "rms_delta_db", "log1p_f0_delta_cents", "voicing_mismatch"}
	for band := 0; band < 10; band++ {
		result = append(result, fmt.Sprintf("spectrum_band_%02d_abs_delta", band))
	}
	result = append(result, "one_minus_waveform_correlation")
	return result
}

func featureVector(features LearningFeatures) []float64 {
	result := []float64{features.SpectrumDelta, features.RMSDelta, math.Log1p(features.F0DeltaCents), 0}
	if features.VoicingMismatch {
		result[3] = 1
	}
	for band := 0; band < 10; band++ {
		value := 0.0
		if band < len(features.PreviousOutgoing.SpectrumDB) && band < len(features.CurrentIncoming.SpectrumDB) {
			value = math.Abs(features.PreviousOutgoing.SpectrumDB[band] - features.CurrentIncoming.SpectrumDB[band])
		}
		result = append(result, value)
	}
	result = append(result, 1-features.WaveformCorrelation)
	return result
}

func normalize(vector, means, scales []float64) {
	for index := range vector {
		vector[index] = (vector[index] - means[index]) / scales[index]
	}
}

func labelCounts(examples []Example) (positive, negative int) {
	for _, example := range examples {
		if example.Label == 1 {
			positive++
		} else {
			negative++
		}
	}
	return positive, negative
}

func voicebanks(examples []Example) []string {
	set := map[string]bool{}
	for _, example := range examples {
		set[example.Voicebank] = true
	}
	result := make([]string, 0, len(set))
	for name := range set {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

func groupFraction(voicebank, group string, seed uint64) float64 {
	hash := fnv.New64a()
	_, _ = fmt.Fprintf(hash, "%d\x00%s\x00%s", seed, voicebank, group)
	return float64(hash.Sum64()%1_000_000) / 1_000_000
}

func sigmoid(value float64) float64 {
	if value >= 0 {
		return 1 / (1 + math.Exp(-value))
	}
	exponential := math.Exp(value)
	return exponential / (1 + exponential)
}

func dotVector(left, right []float64) float64 {
	result := 0.0
	for index := range left {
		result += left[index] * right[index]
	}
	return result
}

func auc(scores []scoredLabel) float64 {
	sort.Slice(scores, func(i, j int) bool { return scores[i].probability < scores[j].probability })
	rankSum := 0.0
	for start := 0; start < len(scores); {
		end := start + 1
		for end < len(scores) && scores[end].probability == scores[start].probability {
			end++
		}
		averageRank := float64(start+1+end) / 2
		for index := start; index < end; index++ {
			if scores[index].label == 1 {
				rankSum += averageRank
			}
		}
		start = end
	}
	positive, negative := 0, 0
	for _, score := range scores {
		if score.label == 1 {
			positive++
		} else {
			negative++
		}
	}
	return (rankSum - float64(positive*(positive+1))/2) / float64(positive*negative)
}
