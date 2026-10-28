package prosody

import (
	"math"
	"strings"

	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauWLD/speechtiming"
)

const (
	FramePitchModelVersion = 8
)

type Model struct {
	ID              string           `json:"id,omitempty"`
	DisplayName     string           `json:"display_name,omitempty"`
	Description     string           `json:"description,omitempty"`
	License         string           `json:"license,omitempty"`
	LicenseNotices  []string         `json:"license_notices,omitempty"`
	Language        string           `json:"language,omitempty"`
	Provenance      *ModelProvenance `json:"provenance,omitempty"`
	DefaultPriority int              `json:"default_priority,omitempty"`
	// F0Headは統合韻律モデル（F0・エネルギーヘッド）。base_modelを持つモデルJSONだけが設定する。
	F0Head *speechtiming.TCN `json:"-"`
	// F0HeadEnergyはF0ヘッドのエネルギーでモーラの音量を変えるか。
	F0HeadEnergy bool `json:"-"`
	// F0HeadPitchOffsetCentsはF0ヘッドの自動ピッチ曲線へ足す高さ（セント）。
	F0HeadPitchOffsetCents float64 `json:"-"`
	// F0HeadBaseBlendは基準モデルの抑揚曲線を混ぜる重み。
	F0HeadBaseBlend float64          `json:"-"`
	Version         int              `json:"version"`
	FeatureVersion  int              `json:"feature_version"`
	Mode            string           `json:"mode"`
	FramePitch      *FramePitchModel `json:"frame_pitch,omitempty"`
	Metrics         Metrics          `json:"metrics"`
	Training        TrainingInfo     `json:"training"`
}

type ModelProvenance struct {
	TrainingCorpus string `json:"training_corpus,omitempty"`
}

type FeatureFrame map[string]float64

type SequencePitchLayer struct {
	Dilation int           `json:"dilation"`
	Weights  [][][]float64 `json:"weights"`
	Bias     []float64     `json:"bias"`
}

type FramePitchModel struct {
	FeatureNames      []string             `json:"feature_names"`
	InputWeights      [][]float64          `json:"input_weights"`
	InputBias         []float64            `json:"input_bias"`
	Layers            []SequencePitchLayer `json:"layers"`
	OutputWeight      []float64            `json:"output_weight"`
	OutputBias        float64              `json:"output_bias"`
	FrameMS           float64              `json:"frame_ms"`
	LowCents          float64              `json:"low_cents"`
	HighCents         float64              `json:"high_cents"`
	RenderStrength    float64              `json:"render_strength,omitempty"`
	RenderSmoothingMS float64              `json:"render_smoothing_ms,omitempty"`
	RenderP99Cents    float64              `json:"render_p99_cents,omitempty"`
	RenderMaxCents    float64              `json:"render_max_cents,omitempty"`

	validated bool
}

type MoraTiming struct {
	StartMS    float64
	DurationMS float64
}

type PitchContour struct {
	FrameMS float64
	Cents   []float64
}

type TrainingInfo struct {
	Records int     `json:"records"`
	Tokens  int     `json:"tokens"`
	Epochs  int     `json:"epochs"`
	Rate    float64 `json:"learning_rate"`
	Seed    int64   `json:"seed"`
}

type Metrics struct {
	Records               int     `json:"records"`
	Tokens                int     `json:"tokens"`
	DurationMAEMS         float64 `json:"normalized_duration_mae_ms"`
	PitchMAECents         float64 `json:"pitch_mae_cents,omitempty"`
	EnergyMAEDB           float64 `json:"energy_mae_db,omitempty"`
	BaselineDurationMAEMS float64 `json:"baseline_duration_mae_ms,omitempty"`
	BaselinePitchMAECents float64 `json:"baseline_pitch_mae_cents,omitempty"`
	BaselineEnergyMAEDB   float64 `json:"baseline_energy_mae_db,omitempty"`
}

func (m *Model) Predict(morae []frontend.Mora) []Prediction {
	return m.PredictWithFeatures(morae, nil)
}

func (m *Model) PredictWithFeatures(morae []frontend.Mora, _ []FeatureFrame) []Prediction {
	result := make([]Prediction, len(morae))
	for i := range result {
		result[i] = Prediction{DurationFactor: 1, PitchFactor: 1, EnergyFactor: 1}
	}
	return result
}

func (m *Model) RequiresExternalFeatures() bool {
	if m.FramePitch == nil {
		return false
	}
	for _, name := range m.FramePitch.FeatureNames {
		if strings.HasPrefix(name, "accent_") || strings.HasPrefix(name, "word_") ||
			strings.HasPrefix(name, "pos=") || strings.HasPrefix(name, "pos_group1=") {
			return true
		}
	}
	return false
}

func (m *Model) HasFrameContour() bool {
	return m != nil && m.FramePitch != nil
}

func (m *Model) PredictFrameContour(morae []frontend.Mora, frames []FeatureFrame, timings []MoraTiming, durationMS float64, question bool) *PitchContour {
	model := m.FramePitch
	if model == nil || len(morae) == 0 || len(timings) != len(morae) || (!model.validated && validateFramePitch(model) != nil) {
		return nil
	}
	count := max(2, int(math.Ceil(durationMS/model.FrameMS))+1)
	featureIndex := make(map[string]int, len(model.FeatureNames))
	for index, name := range model.FeatureNames {
		featureIndex[name] = index
	}
	baseFeatures := indexedFeatureVectors(morae, frames, featureIndex)
	hidden := len(model.InputBias)
	state := make([][]float64, count)
	// TCNと同様、10msフレームごとの小さな確保を一つにまとめる。
	stateValues := make([]float64, count*hidden)
	speech := make([]bool, count)
	moraIndex := 0
	for frameIndex := 0; frameIndex < count; frameIndex++ {
		timeMS := float64(frameIndex) * model.FrameMS
		for moraIndex+1 < len(timings) && timeMS >= timings[moraIndex].StartMS+timings[moraIndex].DurationMS {
			moraIndex++
		}
		duration := math.Max(1, timings[moraIndex].DurationMS)
		progress := clamp((timeMS-timings[moraIndex].StartMS)/duration, 0, 1)
		framePosition := float64(frameIndex) / float64(max(1, count-1))
		state[frameIndex] = stateValues[frameIndex*hidden : (frameIndex+1)*hidden]
		copy(state[frameIndex], model.InputBias)
		for _, feature := range baseFeatures[moraIndex] {
			for output := 0; output < hidden; output++ {
				state[frameIndex][output] += model.InputWeights[output][feature.column] * feature.value
			}
		}
		addIndexedFeature(state[frameIndex], model.InputWeights, featureIndex, "mora_progress", progress)
		addIndexedFeature(state[frameIndex], model.InputWeights, featureIndex, "mora_progress2", progress*progress)
		addIndexedFeature(state[frameIndex], model.InputWeights, featureIndex, "frame_position", framePosition)
		addIndexedFeature(state[frameIndex], model.InputWeights, featureIndex, "frame_from_end", 1-framePosition)
		addIndexedFeature(state[frameIndex], model.InputWeights, featureIndex, "final_distance", 1-framePosition)
		if question {
			addIndexedFeature(state[frameIndex], model.InputWeights, featureIndex, "question_distance", 1-framePosition)
		}
		for output := range state[frameIndex] {
			state[frameIndex][output] = math.Tanh(state[frameIndex][output])
		}
		speech[frameIndex] = !morae[moraIndex].Pause
	}
	state = tcnForward(state, model.Layers, nil)
	values := make([]float64, count)
	var voiced []float64
	for position := range state {
		value := model.OutputBias
		for index, weight := range model.OutputWeight {
			value += weight * state[position][index]
		}
		values[position] = value
		if speech[position] {
			voiced = append(voiced, value)
		}
	}
	center := median(voiced)
	strength := model.RenderStrength
	if strength <= 0 || strength > 1 {
		strength = 1
	}
	for position := range values {
		if !speech[position] {
			values[position] = 0
		} else {
			values[position] -= center
		}
	}
	if model.RenderStrength > 0 {
		smoothingMS := model.RenderSmoothingMS
		if smoothingMS <= 0 {
			smoothingMS = 20
		}
		values = SmoothPitchPhrases(values, speech, model.FrameMS, smoothingMS)
	}
	for position := range values {
		if speech[position] {
			values[position] *= strength
		}
	}
	if model.RenderStrength > 0 {
		p99 := model.RenderP99Cents
		if p99 <= 0 {
			p99 = 75
		}
		maximum := model.RenderMaxCents
		if maximum <= 0 {
			maximum = 90
		}
		ClipPitchContour(values, speech, p99, maximum)
	}
	for position := range values {
		values[position] = clamp(values[position], model.LowCents, model.HighCents)
	}
	contour := &PitchContour{FrameMS: model.FrameMS, Cents: values}
	return contour
}
