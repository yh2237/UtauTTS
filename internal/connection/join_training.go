package connection

import (
	"fmt"
	"math"
)

// JoinTrainingOptions controls the dependency-free offline ranker trainer.
type JoinTrainingOptions struct {
	ID            string
	Description   string
	Epochs        int
	LearningRate  float64
	L2            float64
	Blend         float64
	ScoreScale    float64
	MinConfidence float64
}

// JoinTrainingReport contains only diagnostics, not model parameters.
type JoinTrainingReport struct {
	Examples int
	Positive int
	Negative int
	Accuracy float64
	LogLoss  float64
}

type joinTrainingExample struct {
	values []float64
	label  float64
}

// TrainJoinModel fits a small standardized logistic ranker from listener
// labels in JoinAuditRow. Rows without labels are skipped. The trainer is
// intentionally simple and deterministic so it can run without Python or a
// machine-learning runtime.
func TrainJoinModel(rows []JoinAuditRow, options JoinTrainingOptions) (*JoinModel, JoinTrainingReport, error) {
	options = normalizeTrainingOptions(options)
	examples := make([]joinTrainingExample, 0, len(rows))
	for index, row := range rows {
		if row.Label == nil {
			continue
		}
		label := *row.Label
		if !isFinite(label) || label < 0 || label > 1 {
			return nil, JoinTrainingReport{}, fmt.Errorf("row %d label must be between 0 and 1", index+1)
		}
		examples = append(examples, joinTrainingExample{values: JoinFeatureVector(row.Features), label: label})
	}
	if len(examples) < 4 {
		return nil, JoinTrainingReport{}, fmt.Errorf("need at least 4 labeled join rows, got %d", len(examples))
	}
	positive, negative := 0, 0
	labelSum := 0.0
	for _, example := range examples {
		labelSum += example.label
		if example.label >= 0.5 {
			positive++
		} else {
			negative++
		}
	}
	if positive == 0 || negative == 0 {
		return nil, JoinTrainingReport{}, fmt.Errorf("labeled rows must contain both classes (positive=%d negative=%d)", positive, negative)
	}

	mean := make([]float64, len(joinFeatureNames))
	for _, example := range examples {
		for index, value := range example.values {
			mean[index] += value
		}
	}
	for index := range mean {
		mean[index] /= float64(len(examples))
	}
	scale := make([]float64, len(joinFeatureNames))
	for _, example := range examples {
		for index, value := range example.values {
			delta := value - mean[index]
			scale[index] += delta * delta
		}
	}
	for index := range scale {
		scale[index] = math.Sqrt(scale[index] / float64(len(examples)))
		if scale[index] < 1e-6 {
			scale[index] = 1
		}
	}

	weights := make([]float64, len(joinFeatureNames))
	bias := logit(clamp(labelSum/float64(len(examples)), 0.02, 0.98))
	for epoch := 0; epoch < options.Epochs; epoch++ {
		gradients := make([]float64, len(weights))
		biasGradient := 0.0
		for _, example := range examples {
			value := bias
			for index, feature := range example.values {
				value += weights[index] * (feature - mean[index]) / scale[index]
			}
			prediction := sigmoid(value)
			error := prediction - example.label
			biasGradient += error
			for index, feature := range example.values {
				gradients[index] += error * (feature - mean[index]) / scale[index]
			}
		}
		count := float64(len(examples))
		bias -= options.LearningRate * biasGradient / count
		for index := range weights {
			weights[index] -= options.LearningRate * (gradients[index]/count + options.L2*weights[index])
		}
	}

	model := &JoinModel{
		Version:       JoinModelVersion,
		Kind:          "logistic_join_ranker",
		ID:            options.ID,
		Description:   options.Description,
		FeatureNames:  JoinFeatureNames(),
		Mean:          mean,
		Scale:         scale,
		Weights:       weights,
		Bias:          bias,
		Blend:         options.Blend,
		ScoreScale:    options.ScoreScale,
		MinConfidence: options.MinConfidence,
		Label:         "1 is a preferred or continuous join; 0 is a rejected or discontinuous join",
		Provenance:    "Trained from labeled join-audit rows; verify voicebank permissions before sharing",
	}
	if err := model.Validate(); err != nil {
		return nil, JoinTrainingReport{}, err
	}
	report := JoinTrainingReport{Examples: len(examples), Positive: positive, Negative: negative}
	report.Accuracy, report.LogLoss = evaluateJoinModel(model, examples)
	return model, report, nil
}

func normalizeTrainingOptions(options JoinTrainingOptions) JoinTrainingOptions {
	if options.ID == "" {
		options.ID = "join-ranker-v1"
	}
	if options.Description == "" {
		options.Description = "Dependency-free learned join quality ranker"
	}
	if options.Epochs <= 0 {
		options.Epochs = 1200
	}
	if options.LearningRate <= 0 || !isFinite(options.LearningRate) {
		options.LearningRate = 0.08
	}
	if options.L2 < 0 || !isFinite(options.L2) {
		options.L2 = 0.001
	}
	if options.Blend < 0 || options.Blend > 1 || !isFinite(options.Blend) {
		options.Blend = 0.35
	}
	if options.ScoreScale <= 0 || !isFinite(options.ScoreScale) {
		options.ScoreScale = 8
	}
	if options.MinConfidence < 0 || options.MinConfidence > 1 || !isFinite(options.MinConfidence) {
		options.MinConfidence = 0.08
	}
	return options
}

func evaluateJoinModel(model *JoinModel, examples []joinTrainingExample) (float64, float64) {
	correct, loss := 0, 0.0
	for _, example := range examples {
		logitValue := model.Bias
		for index, value := range example.values {
			logitValue += model.Weights[index] * (value - model.Mean[index]) / model.Scale[index]
		}
		probability := clamp(sigmoid(logitValue), 1e-12, 1-1e-12)
		predicted := probability >= 0.5
		actual := example.label >= 0.5
		if predicted == actual {
			correct++
		}
		loss -= example.label*math.Log(probability) + (1-example.label)*math.Log(1-probability)
	}
	count := float64(len(examples))
	return float64(correct) / count, loss / count
}

func logit(value float64) float64 {
	return math.Log(value / (1 - value))
}

func clamp(value, low, high float64) float64 {
	return math.Max(low, math.Min(high, value))
}
