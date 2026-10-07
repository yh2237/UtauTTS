package prosody

import (
	"math"
	"strings"

	"utautts/internal/frontend"
	"utautts/internal/speechtiming"
)

const ModelVersion = 3

const (
	FramePitchModelVersion         = 8
	ProsodyMultitaskModelVersion   = 10
	ManualResidualModelVersion     = 11
	EnglishIntonationModelVersion  = 12
	MandarinIntonationModelVersion = 13
)

type Model struct {
	ID                   string                   `json:"id,omitempty"`
	DisplayName          string                   `json:"display_name,omitempty"`
	Description          string                   `json:"description,omitempty"`
	License              string                   `json:"license,omitempty"`
	LicenseNotices       []string                 `json:"license_notices,omitempty"`
	Language             string                   `json:"language,omitempty"`
	Provenance           *ModelProvenance         `json:"provenance,omitempty"`
	RecommendedRenderers []string                 `json:"recommended_renderers,omitempty"`
	DefaultPriority      int                      `json:"default_priority,omitempty"`
	// F0Headは統合韻律モデル（F0・エネルギーヘッド）。base_modelを持つモデルJSONだけが設定する。
	F0Head *speechtiming.TCN `json:"-"`
	Version              int                      `json:"version"`
	FeatureVersion       int                      `json:"feature_version"`
	Mode                 string                   `json:"mode"`
	Outputs              map[string]bool          `json:"outputs,omitempty"`
	DurationWeights      map[string]float64       `json:"duration_weights"`
	PitchWeights         map[string]float64       `json:"pitch_weights,omitempty"`
	EnergyWeights        map[string]float64       `json:"energy_weights,omitempty"`
	MoraDuration         *SequencePitchModel      `json:"mora_duration,omitempty"`
	FramePitch           *FramePitchModel         `json:"frame_pitch,omitempty"`
	MoraPitchResidual    *MoraPitchResidualModel  `json:"mora_pitch_residual,omitempty"`
	EnglishIntonation    *EnglishIntonationModel  `json:"english_intonation,omitempty"`
	MandarinIntonation   *MandarinIntonationModel `json:"mandarin_intonation,omitempty"`
	BaseModel            *BaseModelReference      `json:"base_model,omitempty"`
	ResidualLimits       *ResidualLimits          `json:"residual_limits,omitempty"`
	Metrics              Metrics                  `json:"metrics"`
	Training             TrainingInfo             `json:"training"`
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

// 英語の強勢と句境界を予測する軽量モデル。
type EnglishIntonationModel struct {
	FrameMS                   float64 `json:"frame_ms"`
	BaselineStartCents        float64 `json:"baseline_start_cents"`
	BaselineEndCents          float64 `json:"baseline_end_cents"`
	PrimaryStressCents        float64 `json:"primary_stress_cents"`
	SecondaryStressCents      float64 `json:"secondary_stress_cents"`
	UnstressedCents           float64 `json:"unstressed_cents"`
	PreStressDipCents         float64 `json:"pre_stress_dip_cents"`
	WordDownstepCents         float64 `json:"word_downstep_cents"`
	PhraseFinalFallCents      float64 `json:"phrase_final_fall_cents"`
	QuestionRiseCents         float64 `json:"question_rise_cents"`
	SmoothingMS               float64 `json:"smoothing_ms"`
	LowCents                  float64 `json:"low_cents"`
	HighCents                 float64 `json:"high_cents"`
	P99Cents                  float64 `json:"p99_cents"`
	MaxCents                  float64 `json:"max_cents"`
	PrimaryDurationFactor     float64 `json:"primary_duration_factor"`
	SecondaryDurationFactor   float64 `json:"secondary_duration_factor"`
	UnstressedDurationFactor  float64 `json:"unstressed_duration_factor"`
	PhraseFinalDurationFactor float64 `json:"phrase_final_duration_factor"`
	PrimaryEnergyFactor       float64 `json:"primary_energy_factor,omitempty"`
	SecondaryEnergyFactor     float64 `json:"secondary_energy_factor,omitempty"`
	UnstressedEnergyFactor    float64 `json:"unstressed_energy_factor,omitempty"`
	PhraseFinalEnergyFactor   float64 `json:"phrase_final_energy_factor,omitempty"`
}

func (model *EnglishIntonationModel) predict(morae []frontend.Mora) []Prediction {
	result := make([]Prediction, len(morae))
	for index, mora := range morae {
		result[index] = Prediction{DurationFactor: 1, PitchFactor: 1, EnergyFactor: 1}
		if mora.Pause {
			continue
		}
		factor := 1.0
		energy := 1.0
		if mora.Vowel != "" {
			switch mora.Stress {
			case 1:
				factor = model.PrimaryDurationFactor
				energy = positiveFactor(model.PrimaryEnergyFactor)
			case 2:
				factor = model.SecondaryDurationFactor
				energy = positiveFactor(model.SecondaryEnergyFactor)
			case 0:
				if mora.StressKnown {
					factor = model.UnstressedDurationFactor
					energy = positiveFactor(model.UnstressedEnergyFactor)
				}
			}
		}
		if mora.WordEnd && (index+1 == len(morae) || morae[index+1].Pause) {
			factor *= model.PhraseFinalDurationFactor
			energy *= positiveFactor(model.PhraseFinalEnergyFactor)
		}
		result[index].DurationFactor = factor
		result[index].EnergyFactor = energy
	}
	return result
}

func positiveFactor(value float64) float64 {
	if value > 0 {
		return value
	}
	return 1
}

func (model *EnglishIntonationModel) predictContour(morae []frontend.Mora, timings []MoraTiming, durationMS float64, question bool) *PitchContour {
	if len(morae) == 0 || len(timings) != len(morae) || durationMS <= 0 || validateEnglishIntonation(model) != nil {
		return nil
	}
	count := max(2, int(math.Ceil(durationMS/model.FrameMS))+1)
	values := make([]float64, count)
	speech := make([]bool, count)
	moraAtFrame := make([]int, count)
	phraseStart := make([]int, 0)
	phraseEnd := make([]int, 0)
	for start := 0; start < len(morae); {
		if morae[start].Pause {
			start++
			continue
		}
		end := start
		for end+1 < len(morae) && !morae[end+1].Pause {
			end++
		}
		phraseStart = append(phraseStart, start)
		phraseEnd = append(phraseEnd, end)
		start = end + 1
	}
	phraseForMora := make([]int, len(morae))
	for phrase, start := range phraseStart {
		for index := start; index <= phraseEnd[phrase]; index++ {
			phraseForMora[index] = phrase
		}
	}
	moraIndex := 0
	for frame := 0; frame < count; frame++ {
		timeMS := float64(frame) * model.FrameMS
		for moraIndex+1 < len(timings) && timeMS >= timings[moraIndex].StartMS+timings[moraIndex].DurationMS {
			moraIndex++
		}
		moraAtFrame[frame] = moraIndex
		speech[frame] = !morae[moraIndex].Pause
	}
	for frame, index := range moraAtFrame {
		if !speech[frame] {
			continue
		}
		phrase := phraseForMora[index]
		start, end := phraseStart[phrase], phraseEnd[phrase]
		left := timings[start].StartMS
		right := timings[end].StartMS + timings[end].DurationMS
		phraseProgress := clamp((float64(frame)*model.FrameMS-left)/math.Max(1, right-left), 0, 1)
		value := model.BaselineStartCents*(1-phraseProgress) + model.BaselineEndCents*phraseProgress
		wordOrdinal := 0
		previousWord := -1
		for position := start; position <= index; position++ {
			if position == start || morae[position].WordIndex != previousWord {
				wordOrdinal++
				previousWord = morae[position].WordIndex
			}
		}
		if wordOrdinal > 1 {
			value -= model.WordDownstepCents * float64(wordOrdinal-1)
		}
		mora := morae[index]
		if mora.Vowel != "" {
			timing := timings[index]
			local := clamp((float64(frame)*model.FrameMS-timing.StartMS)/math.Max(1, timing.DurationMS), 0, 1)
			var stressCents float64
			switch mora.Stress {
			case 1:
				stressCents = model.PrimaryStressCents
			case 2:
				stressCents = model.SecondaryStressCents
			case 0:
				if mora.StressKnown {
					stressCents = model.UnstressedCents
				}
			}
			if stressCents != 0 {
				value += stressCents * math.Sin(math.Pi*local)
			}
			if mora.Stress > 0 && local < 0.28 {
				value += model.PreStressDipCents * (1 - local/0.28) / float64(mora.Stress)
			}
		}
		if index == end && phraseProgress > 0.64 {
			boundary := smoothstep01((phraseProgress - 0.64) / 0.36)
			if question && phrase == len(phraseStart)-1 {
				value += model.QuestionRiseCents * boundary
			} else {
				value += model.PhraseFinalFallCents * boundary
			}
		}
		values[frame] = value
	}
	values = smoothFramePitchPhrases(values, speech, model.FrameMS, model.SmoothingMS)
	voiced := make([]float64, 0, count)
	for index, value := range values {
		if speech[index] {
			voiced = append(voiced, value)
		}
	}
	center := median(voiced)
	for index := range values {
		if speech[index] {
			values[index] -= center
		} else {
			values[index] = 0
		}
	}
	if observed := absolutePercentile(values, speech, 0.99); observed > model.P99Cents {
		gain := model.P99Cents / observed
		for index := range values {
			if speech[index] {
				values[index] *= gain
			}
		}
	}
	for index := range values {
		values[index] = clamp(values[index], model.LowCents, model.HighCents)
		if speech[index] {
			values[index] = clamp(values[index], -model.MaxCents, model.MaxCents)
		}
	}
	return &PitchContour{FrameMS: model.FrameMS, Cents: values}
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
	if m.EnglishIntonation != nil {
		return m.EnglishIntonation.predict(morae)
	}
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
	if m.EnglishIntonation != nil {
		return false
	}
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
	return m != nil && (m.FramePitch != nil || m.EnglishIntonation != nil)
}

func (m *Model) PredictFrameContour(morae []frontend.Mora, frames []FeatureFrame, timings []MoraTiming, durationMS float64, question bool) *PitchContour {
	if m.EnglishIntonation != nil {
		return m.EnglishIntonation.predictContour(morae, timings, durationMS, question)
	}
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
		state[frameIndex] = append([]float64(nil), model.InputBias...)
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
		values = smoothFramePitchPhrases(values, speech, model.FrameMS, smoothingMS)
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
		if observed := absolutePercentile(values, speech, 0.99); observed > p99 {
			gain := p99 / observed
			for position := range values {
				if speech[position] {
					values[position] *= gain
				}
			}
		}
		maximum := model.RenderMaxCents
		if maximum <= 0 {
			maximum = 90
		}
		for position := range values {
			values[position] = clamp(values[position], -maximum, maximum)
		}
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
