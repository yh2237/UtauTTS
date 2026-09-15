package jsut

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
)

type TransitionTCN struct {
	Version      int                        `json:"version"`
	Kind         string                     `json:"kind"`
	ID           string                     `json:"id"`
	PositionBins int                        `json:"position_bins"`
	Phones       []string                   `json:"phones"`
	InputSize    int                        `json:"input_size"`
	HiddenSize   int                        `json:"hidden_size"`
	OutputSize   int                        `json:"output_size"`
	OutputScale  float64                    `json:"output_scale"`
	Layers       map[string]TransitionLayer `json:"layers"`
}

type TransitionLayer struct {
	Weight [][][]float64 `json:"weight"`
	Bias   []float64     `json:"bias"`
}

type TransitionPrediction struct {
	RMSResidualDB      float64
	SpectrumResidualDB []float64
}

func LoadTransitionTCN(path string) (*TransitionTCN, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var model TransitionTCN
	if err := json.Unmarshal(data, &model); err != nil {
		return nil, err
	}
	if err := model.Validate(); err != nil {
		return nil, err
	}
	return &model, nil
}

func (model *TransitionTCN) Validate() error {
	if model == nil || model.Version != 1 || model.Kind != "jsut_cv_transition_tcn" || model.PositionBins < 3 || model.OutputScale <= 0 || model.OutputSize < 2 {
		return fmt.Errorf("invalid transition TCN metadata")
	}
	expected := map[string][3]int{"input": {model.HiddenSize, model.InputSize, 1}, "conv1": {model.HiddenSize, model.HiddenSize, 3}, "conv2": {model.HiddenSize, model.HiddenSize, 3}, "output": {model.OutputSize, model.HiddenSize, 1}}
	for name, shape := range expected {
		layer, ok := model.Layers[name]
		if !ok || len(layer.Weight) != shape[0] || len(layer.Bias) != shape[0] {
			return fmt.Errorf("invalid transition layer %s", name)
		}
		for _, output := range layer.Weight {
			if len(output) != shape[1] {
				return fmt.Errorf("invalid transition layer %s inputs", name)
			}
			for _, kernel := range output {
				if len(kernel) != shape[2] {
					return fmt.Errorf("invalid transition layer %s kernel", name)
				}
			}
		}
	}
	return nil
}

func (model *TransitionTCN) Predict(leftPhone, rightPhone string, left, right Frame) ([]TransitionPrediction, bool) {
	if model == nil || model.Validate() != nil || len(left.SpectrumDB) != 10 || len(right.SpectrumDB) != 10 {
		return nil, false
	}
	phoneIndex := make(map[string]int, len(model.Phones))
	for index, phone := range model.Phones {
		phoneIndex[phone] = index
	}
	li, lok := phoneIndex[leftPhone]
	ri, rok := phoneIndex[rightPhone]
	if !lok || !rok {
		return nil, false
	}
	input := make([][]float64, model.InputSize)
	for channel := range input {
		input[channel] = make([]float64, model.PositionBins)
	}
	base := []float64{left.RMSDB / 60, right.RMSDB / 60, math.Log(math.Max(left.F0Hz, 1)) / 7, math.Log(math.Max(right.F0Hz, 1)) / 7, boolNumber(left.F0Hz > 0), boolNumber(right.F0Hz > 0)}
	for _, values := range [][]float64{left.SpectrumDB, right.SpectrumDB} {
		for _, value := range values {
			base = append(base, value/30)
		}
	}
	for position := 0; position < model.PositionBins; position++ {
		for channel, value := range base {
			input[channel][position] = value
		}
		input[len(base)+li][position] = 1
		input[len(base)+len(model.Phones)+ri][position] = 1
		p := float64(position) / float64(model.PositionBins-1)
		input[len(base)+2*len(model.Phones)][position] = p
		input[len(base)+2*len(model.Phones)+1][position] = p * p
		input[len(base)+2*len(model.Phones)+2][position] = p * p * p
	}
	h := reluTransition(convolveTransition(input, model.Layers["input"]))
	h = addTransition(h, reluTransition(convolveTransition(h, model.Layers["conv1"])))
	h = addTransition(h, reluTransition(convolveTransition(h, model.Layers["conv2"])))
	output := convolveTransition(h, model.Layers["output"])
	result := make([]TransitionPrediction, model.PositionBins)
	for position := range result {
		result[position].RMSResidualDB = output[0][position] * model.OutputScale
		result[position].SpectrumResidualDB = make([]float64, model.OutputSize-1)
		for band := range result[position].SpectrumResidualDB {
			result[position].SpectrumResidualDB[band] = output[band+1][position] * model.OutputScale
		}
	}
	return result, true
}

func convolveTransition(input [][]float64, layer TransitionLayer) [][]float64 {
	positions := len(input[0])
	result := make([][]float64, len(layer.Weight))
	padding := len(layer.Weight[0][0]) / 2
	for out := range result {
		result[out] = make([]float64, positions)
		for position := 0; position < positions; position++ {
			value := layer.Bias[out]
			for in := range input {
				for kernel, weight := range layer.Weight[out][in] {
					source := position + kernel - padding
					if source >= 0 && source < positions {
						value += input[in][source] * weight
					}
				}
			}
			result[out][position] = value
		}
	}
	return result
}

func reluTransition(values [][]float64) [][]float64 {
	for channel := range values {
		for position := range values[channel] {
			values[channel][position] = math.Max(0, values[channel][position])
		}
	}
	return values
}

func addTransition(left, right [][]float64) [][]float64 {
	for channel := range left {
		for position := range left[channel] {
			left[channel][position] += right[channel][position]
		}
	}
	return left
}

func boolNumber(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
