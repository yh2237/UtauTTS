package prosody

import (
	"math"
	"strings"

	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/speechtiming"
)

const ModelVersion = 3

const (
	FramePitchModelVersion         = 8
	ProsodyMultitaskModelVersion   = 10
	ManualResidualModelVersion     = 11
	MandarinIntonationModelVersion = 13
)

type Model struct {
	ID                   string           `json:"id,omitempty"`
	DisplayName          string           `json:"display_name,omitempty"`
	Description          string           `json:"description,omitempty"`
	License              string           `json:"license,omitempty"`
	LicenseNotices       []string         `json:"license_notices,omitempty"`
	Language             string           `json:"language,omitempty"`
	Provenance           *ModelProvenance `json:"provenance,omitempty"`
	RecommendedRenderers []string         `json:"recommended_renderers,omitempty"`
	DefaultPriority      int              `json:"default_priority,omitempty"`
	// F0Headは統合韻律モデル（F0・エネルギーヘッド）。base_modelを持つモデルJSONだけが設定する。
	F0Head *speechtiming.TCN `json:"-"`
	// F0HeadEnergyはF0ヘッドのエネルギーでモーラの音量を変えるか。
	F0HeadEnergy bool `json:"-"`
	// F0HeadPitchOffsetCentsはF0ヘッドの自動ピッチ曲線へ足す高さ（セント）。
	F0HeadPitchOffsetCents float64 `json:"-"`
	// F0HeadBaseBlendは基準モデルの抑揚曲線を混ぜる重み。
	F0HeadBaseBlend    float64                  `json:"-"`
	Version            int                      `json:"version"`
	FeatureVersion     int                      `json:"feature_version"`
	Mode               string                   `json:"mode"`
	Outputs            map[string]bool          `json:"outputs,omitempty"`
	DurationWeights    map[string]float64       `json:"duration_weights"`
	PitchWeights       map[string]float64       `json:"pitch_weights,omitempty"`
	EnergyWeights      map[string]float64       `json:"energy_weights,omitempty"`
	MoraDuration       *SequencePitchModel      `json:"mora_duration,omitempty"`
	FramePitch         *FramePitchModel         `json:"frame_pitch,omitempty"`
	MoraPitchResidual  *MoraPitchResidualModel  `json:"mora_pitch_residual,omitempty"`
	MandarinIntonation *MandarinIntonationModel `json:"mandarin_intonation,omitempty"`
	BaseModel          *BaseModelReference      `json:"base_model,omitempty"`
	ResidualLimits     *ResidualLimits          `json:"residual_limits,omitempty"`
	Metrics            Metrics                  `json:"metrics"`
	Training           TrainingInfo             `json:"training"`
}

type ModelProvenance struct {
	TrainingCorpus string `json:"training_corpus,omitempty"`
}

type BaseModelReference struct {
	ID     string `json:"id"`
	SHA256 string `json:"sha256"`
}

type ResidualLimits struct {
	LowCents    float64 `json:"low_cents"`
	HighCents   float64 `json:"high_cents"`
	SmoothingMS float64 `json:"smoothing_ms,omitempty"`
}

type FeatureFrame map[string]float64

type SequencePitchModel struct {
	FeatureNames []string             `json:"feature_names"`
	InputWeights [][]float64          `json:"input_weights"`
	InputBias    []float64            `json:"input_bias"`
	Layers       []SequencePitchLayer `json:"layers"`
	OutputWeight []float64            `json:"output_weight"`
	OutputBias   float64              `json:"output_bias"`
	Low          float64              `json:"low"`
	High         float64              `json:"high"`
	// 語尾が欠けないよう句末の長さに下限を置く。0は無効。
	PhraseFinalLow float64 `json:"phrase_final_low,omitempty"`
	// 整列で文頭に無音・息が混ざるため、長さに上限を置く。0は無効。
	PhraseStartHigh float64 `json:"phrase_start_high,omitempty"`

	validated bool
}

type SequencePitchLayer struct {
	Dilation int           `json:"dilation"`
	Weights  [][][]float64 `json:"weights"`
	Bias     []float64     `json:"bias"`
}

type MoraPitchResidualModel struct {
	FeatureNames []string             `json:"feature_names"`
	InputWeights [][]float64          `json:"input_weights"`
	InputBias    []float64            `json:"input_bias"`
	Layers       []SequencePitchLayer `json:"layers"`
	OutputWeight []float64            `json:"output_weight"`
	OutputBias   float64              `json:"output_bias"`
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

type MandarinIntonationModel struct {
	FeatureNames []string    `json:"feature_names"`
	Knots        []float64   `json:"knots"`
	Weights      [][]float64 `json:"weights"`
	Strength     float64     `json:"strength"`
	MaxCents     float64     `json:"max_cents"`
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

func (m *Model) PredictWithFeatures(morae []frontend.Mora, frames []FeatureFrame) []Prediction {
	result := make([]Prediction, len(morae))
	durations := centeredFactors(m.DurationWeights, morae, 0.8, 1.25)
	if m.MoraDuration != nil {
		durations = m.MoraDuration.predict(morae, frames)
	}
	pitches := centeredFactors(m.PitchWeights, morae, 0.97, 1.03)
	energies := centeredFactors(m.EnergyWeights, morae, 0.9, 1.1)
	for i := range morae {
		result[i] = Prediction{
			DurationFactor: durations[i],
			PitchFactor:    pitches[i],
			EnergyFactor:   energies[i],
		}
	}
	return result
}

func (m *Model) RequiresExternalFeatures() bool {
	featureSets := [][]string(nil)
	if m.MoraDuration != nil {
		featureSets = append(featureSets, m.MoraDuration.FeatureNames)
	}
	if m.FramePitch != nil {
		featureSets = append(featureSets, m.FramePitch.FeatureNames)
	}
	if m.MoraPitchResidual != nil {
		featureSets = append(featureSets, m.MoraPitchResidual.FeatureNames)
	}
	for _, names := range featureSets {
		for _, name := range names {
			if strings.HasPrefix(name, "accent_") || strings.HasPrefix(name, "word_") ||
				strings.HasPrefix(name, "pos=") || strings.HasPrefix(name, "pos_group1=") {
				return true
			}
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
	if m.MoraPitchResidual != nil {
		return m.applyMoraPitchResidual(contour, morae, frames, timings)
	}
	return contour
}

func (m *SequencePitchModel) predict(morae []frontend.Mora, frames []FeatureFrame) []float64 {
	result := make([]float64, len(morae))
	for i := range result {
		result[i] = 1
	}
	if len(morae) == 0 || (!m.validated && validateSequencePitch(m) != nil) {
		return result
	}
	featureIndex := make(map[string]int, len(m.FeatureNames))
	for i, name := range m.FeatureNames {
		featureIndex[name] = i
	}
	baseFeatures := indexedFeatureVectors(morae, frames, featureIndex)
	hidden := len(m.InputBias)
	state := make([][]float64, len(morae))
	for position := range morae {
		state[position] = append([]float64(nil), m.InputBias...)
		for _, feature := range baseFeatures[position] {
			for output := 0; output < hidden; output++ {
				state[position][output] += m.InputWeights[output][feature.column] * feature.value
			}
		}
		for output := range state[position] {
			state[position][output] = math.Tanh(state[position][output])
		}
	}
	state = tcnForward(state, m.Layers, nil)
	logs := make([]float64, len(morae))
	var speech []float64
	for position := range morae {
		if morae[position].Pause {
			continue
		}
		value := m.OutputBias
		for i, weight := range m.OutputWeight {
			value += weight * state[position][i]
		}
		logs[position] = value
		speech = append(speech, value)
	}
	if len(speech) == 0 {
		return result
	}
	center := median(speech)
	for position := range morae {
		if !morae[position].Pause {
			result[position] = clamp(math.Exp(logs[position]-center), m.Low, m.High)
			if m.PhraseStartHigh > 0 && (position == 0 || morae[position-1].Pause) {
				result[position] = math.Min(result[position], m.PhraseStartHigh)
			}
			if m.PhraseFinalLow > 0 && (position+1 == len(morae) || morae[position+1].Pause) {
				result[position] = math.Max(result[position], m.PhraseFinalLow)
			}
		}
	}
	return result
}

func centeredFactors(weights map[string]float64, morae []frontend.Mora, low, high float64) []float64 {
	result := make([]float64, len(morae))
	logs := make([]float64, len(morae))
	var speech []float64
	for i := range morae {
		result[i] = 1
		if morae[i].Pause || len(weights) == 0 {
			continue
		}
		logs[i] = dot(weights, featuresFor(morae, i))
		speech = append(speech, logs[i])
	}
	if len(speech) == 0 {
		return result
	}
	center := median(speech)
	for i := range morae {
		if !morae[i].Pause {
			result[i] = clamp(math.Exp(logs[i]-center), low, high)
		}
	}
	return result
}
