package prosody

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"utautts/internal/frontend"
)

const ModelVersion = 3

const (
	FramePitchModelVersion        = 8
	ProsodyMultitaskModelVersion  = 10
	ManualResidualModelVersion    = 11
	EnglishIntonationModelVersion = 12
)

type Model struct {
	ID                   string             `json:"id,omitempty"`
	DisplayName          string             `json:"display_name,omitempty"`
	Description          string             `json:"description,omitempty"`
	Language             string             `json:"language,omitempty"`
	Provenance           *ModelProvenance   `json:"provenance,omitempty"`
	RecommendedRenderers []string           `json:"recommended_renderers,omitempty"`
	DefaultPriority      int                `json:"default_priority,omitempty"`
	Version              int                `json:"version"`
	FeatureVersion       int                `json:"feature_version"`
	Mode                 string             `json:"mode"`
	Outputs              map[string]bool    `json:"outputs,omitempty"`
	DurationWeights      map[string]float64 `json:"duration_weights"`
	PitchWeights         map[string]float64 `json:"pitch_weights,omitempty"`
	EnergyWeights        map[string]float64 `json:"energy_weights,omitempty"`
	// MoraDurationはモーラ長の倍率を出すマルチタスクモデルの継続時間ヘッド。
	MoraDuration      *SequencePitchModel     `json:"mora_duration,omitempty"`
	FramePitch        *FramePitchModel        `json:"frame_pitch,omitempty"`
	MoraPitchResidual *MoraPitchResidualModel `json:"mora_pitch_residual,omitempty"`
	EnglishIntonation *EnglishIntonationModel `json:"english_intonation,omitempty"`
	BaseModel         *BaseModelReference     `json:"base_model,omitempty"`
	ResidualLimits    *ResidualLimits         `json:"residual_limits,omitempty"`
	Metrics           Metrics                 `json:"metrics"`
	Training          TrainingInfo            `json:"training"`
}

type ModelProvenance struct {
	TrainingCorpus        string `json:"training_corpus,omitempty"`
	TrainingCorpusLicense string `json:"training_corpus_license,omitempty"`
	SourceNotice          string `json:"source_notice,omitempty"`
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
}

type SequencePitchLayer struct {
	Dilation int           `json:"dilation"`
	Weights  [][][]float64 `json:"weights"`
	Bias     []float64     `json:"bias"`
}

// MoraPitchResidualModelはモーラごとの補正centを出力する。
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
}

func (model *EnglishIntonationModel) predict(morae []frontend.Mora) []Prediction {
	result := make([]Prediction, len(morae))
	for index, mora := range morae {
		result[index] = Prediction{DurationFactor: 1, PitchFactor: 1, EnergyFactor: 1}
		if mora.Pause {
			continue
		}
		factor := 1.0
		if mora.Vowel != "" {
			switch mora.Stress {
			case 1:
				factor = model.PrimaryDurationFactor
			case 2:
				factor = model.SecondaryDurationFactor
			case 0:
				if mora.StressKnown {
					factor = model.UnstressedDurationFactor
				}
			}
		}
		if mora.WordEnd && (index+1 == len(morae) || morae[index+1].Pause) {
			factor *= model.PhraseFinalDurationFactor
		}
		result[index].DurationFactor = factor
	}
	return result
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

func LoadModel(path string) (*Model, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var model Model
	if err := json.Unmarshal(data, &model); err != nil {
		return nil, err
	}
	current := model.Version == ModelVersion && model.FeatureVersion == 1 && model.Mode == "speech_prosody_residual"
	frame := model.FeatureVersion == 1 && model.Version == FramePitchModelVersion && model.Mode == "intonation_frame_tcn_accent_bounded"
	multitask := model.FeatureVersion == 2 && model.Version == ProsodyMultitaskModelVersion && model.Mode == "prosody_multitask_tcn"
	manualResidual := model.FeatureVersion == 2 && model.Version == ManualResidualModelVersion && model.Mode == "intonation_frame_v8_manual_residual"
	englishIntonation := model.FeatureVersion == 1 && model.Version == EnglishIntonationModelVersion && model.Mode == "english_intonation_v1"
	if !current && !frame && !multitask && !manualResidual && !englishIntonation {
		return nil, fmt.Errorf("unsupported prosody model version %d/feature %d mode %q", model.Version, model.FeatureVersion, model.Mode)
	}
	var allowedHeads []string
	switch {
	case current:
		allowedHeads = []string{}
		if err := validateWeightMap("duration_weights", model.DurationWeights); err != nil {
			return nil, err
		}
		if err := validateWeightMap("pitch_weights", model.PitchWeights); err != nil {
			return nil, err
		}
		if err := validateWeightMap("energy_weights", model.EnergyWeights); err != nil {
			return nil, err
		}
	case frame:
		allowedHeads = []string{"frame_pitch"}
	case multitask:
		allowedHeads = []string{"mora_duration", "frame_pitch"}
	case manualResidual:
		allowedHeads = []string{"frame_pitch", "mora_pitch_residual"}
	case englishIntonation:
		allowedHeads = []string{"english_intonation"}
	}
	for _, head := range model.heads() {
		if head.present && !containsString(allowedHeads, head.name) {
			return nil, fmt.Errorf("prosody model head %q is not supported by mode %q", head.name, model.Mode)
		}
	}
	if frame {
		if err := validateFramePitch(model.FramePitch); err != nil {
			return nil, fmt.Errorf("invalid frame pitch model: %w", err)
		}
	}
	if multitask {
		if err := validateSequencePitch(model.MoraDuration); err != nil {
			return nil, fmt.Errorf("invalid mora duration model: %w", err)
		}
		if err := validateFramePitch(model.FramePitch); err != nil {
			return nil, fmt.Errorf("invalid multitask frame pitch model: %w", err)
		}
	}
	if manualResidual {
		if err := validateFramePitch(model.FramePitch); err != nil {
			return nil, fmt.Errorf("invalid manual residual frame pitch model: %w", err)
		}
		if err := validateMoraPitchResidual(model.MoraPitchResidual, model.ResidualLimits); err != nil {
			return nil, fmt.Errorf("invalid mora pitch residual model: %w", err)
		}
		if model.BaseModel == nil || model.BaseModel.ID == "" || len(model.BaseModel.SHA256) != 64 {
			return nil, fmt.Errorf("invalid manual residual base model metadata")
		}
	}
	if englishIntonation {
		if err := validateEnglishIntonation(model.EnglishIntonation); err != nil {
			return nil, fmt.Errorf("invalid English intonation model: %w", err)
		}
		if !strings.EqualFold(strings.TrimSpace(model.Language), "en") {
			return nil, fmt.Errorf("English intonation model must declare language en")
		}
	}
	return &model, nil
}

func (m *Model) Save(path string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if directory := filepath.Dir(path); directory != "." {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, data, 0o644)
}

type modelHead struct {
	name    string
	present bool
}

func (m *Model) heads() []modelHead {
	return []modelHead{
		{"mora_duration", m.MoraDuration != nil},
		{"frame_pitch", m.FramePitch != nil},
		{"mora_pitch_residual", m.MoraPitchResidual != nil},
		{"english_intonation", m.EnglishIntonation != nil},
	}
}

// SupportsLanguageはモデルの対象言語を判定する。未指定の旧モデルは日本語とする。
func (m *Model) SupportsLanguage(language string) bool {
	if m == nil {
		return false
	}
	declared := strings.TrimSpace(m.Language)
	if declared == "" {
		return language == frontend.LanguageJapanese
	}
	return strings.EqualFold(declared, strings.TrimSpace(language))
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func validateWeightMap(name string, weights map[string]float64) error {
	for key, value := range weights {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("%s %q must be finite, got %v", name, key, value)
		}
	}
	return nil
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

// RequiresExternalFeaturesはGoフロントエンドだけでは得られない入力の有無を返す。
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

// HasFrameContourはモデルがフレームピッチ曲線を生成するかを返す。
func (m *Model) HasFrameContour() bool {
	return m != nil && (m.FramePitch != nil || m.EnglishIntonation != nil)
}

func validateEnglishIntonation(model *EnglishIntonationModel) error {
	if model == nil || model.FrameMS < 1 || model.LowCents >= model.HighCents ||
		model.LowCents > 0 || model.HighCents < 0 || model.P99Cents <= 0 || model.MaxCents <= 0 ||
		model.SmoothingMS < 0 || model.PrimaryStressCents <= 0 || model.SecondaryStressCents < 0 ||
		model.UnstressedCents > 0 || model.PreStressDipCents > 0 || model.WordDownstepCents < 0 ||
		model.PhraseFinalFallCents > 0 || model.QuestionRiseCents < 0 {
		return fmt.Errorf("invalid metadata")
	}
	for name, value := range map[string]float64{
		"baseline_start_cents": model.BaselineStartCents, "baseline_end_cents": model.BaselineEndCents,
		"primary_stress_cents": model.PrimaryStressCents, "secondary_stress_cents": model.SecondaryStressCents,
		"unstressed_cents": model.UnstressedCents, "pre_stress_dip_cents": model.PreStressDipCents,
		"word_downstep_cents": model.WordDownstepCents, "phrase_final_fall_cents": model.PhraseFinalFallCents,
		"question_rise_cents": model.QuestionRiseCents, "smoothing_ms": model.SmoothingMS,
		"low_cents": model.LowCents, "high_cents": model.HighCents, "p99_cents": model.P99Cents,
		"max_cents": model.MaxCents, "primary_duration_factor": model.PrimaryDurationFactor,
		"secondary_duration_factor": model.SecondaryDurationFactor, "unstressed_duration_factor": model.UnstressedDurationFactor,
		"phrase_final_duration_factor": model.PhraseFinalDurationFactor,
	} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("%s must be finite", name)
		}
	}
	for name, value := range map[string]float64{
		"primary_duration_factor": model.PrimaryDurationFactor, "secondary_duration_factor": model.SecondaryDurationFactor,
		"unstressed_duration_factor": model.UnstressedDurationFactor, "phrase_final_duration_factor": model.PhraseFinalDurationFactor,
	} {
		if value <= 0 {
			return fmt.Errorf("%s must be positive", name)
		}
	}
	return nil
}

func validateFramePitch(model *FramePitchModel) error {
	if model == nil || model.FrameMS < 1 || model.LowCents >= model.HighCents || model.LowCents > 0 || model.HighCents < 0 {
		return fmt.Errorf("invalid frame pitch metadata")
	}
	portable := &SequencePitchModel{
		FeatureNames: model.FeatureNames, InputWeights: model.InputWeights, InputBias: model.InputBias,
		Layers: model.Layers, OutputWeight: model.OutputWeight, OutputBias: model.OutputBias,
		Low: 0.01, High: 100,
	}
	return validateSequencePitch(portable)
}

func (m *Model) PredictFrameContour(morae []frontend.Mora, frames []FeatureFrame, timings []MoraTiming, durationMS float64, question bool) *PitchContour {
	if m.EnglishIntonation != nil {
		return m.EnglishIntonation.predictContour(morae, timings, durationMS, question)
	}
	model := m.FramePitch
	if model == nil || len(morae) == 0 || len(timings) != len(morae) || validateFramePitch(model) != nil {
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
	for _, layer := range model.Layers {
		next := make([][]float64, len(state))
		for position := range state {
			next[position] = make([]float64, hidden)
			for output := 0; output < hidden; output++ {
				value := state[position][output] + layer.Bias[output]
				for input := 0; input < hidden; input++ {
					for kernel := 0; kernel < 3; kernel++ {
						source := position + (kernel-1)*layer.Dilation
						if source >= 0 && source < len(state) {
							value += layer.Weights[output][input][kernel] * state[source][input]
						}
					}
				}
				next[position][output] = math.Tanh(value)
			}
		}
		state = next
	}
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

func smoothstep01(value float64) float64 {
	value = clamp(value, 0, 1)
	return value * value * (3 - 2*value)
}

func smoothFramePitchPhrases(values []float64, speech []bool, frameMS, sigmaMS float64) []float64 {
	result := append([]float64(nil), values...)
	if frameMS <= 0 || sigmaMS <= 0 || len(values) != len(speech) {
		return result
	}
	sigma := sigmaMS / frameMS
	radius := max(1, int(math.Ceil(3*sigma)))
	weights := make([]float64, 2*radius+1)
	for offset := -radius; offset <= radius; offset++ {
		weights[offset+radius] = math.Exp(-0.5 * math.Pow(float64(offset)/sigma, 2))
	}
	for start := 0; start < len(values); {
		if !speech[start] {
			start++
			continue
		}
		end := start + 1
		for end < len(values) && speech[end] {
			end++
		}
		for position := start; position < end; position++ {
			sum, weightSum := 0.0, 0.0
			for offset := -radius; offset <= radius; offset++ {
				source := max(start, min(end-1, position+offset))
				weight := weights[offset+radius]
				sum += values[source] * weight
				weightSum += weight
			}
			result[position] = sum / weightSum
		}
		start = end
	}
	return result
}

func absolutePercentile(values []float64, mask []bool, quantile float64) float64 {
	selected := make([]float64, 0, len(values))
	for index, value := range values {
		if index < len(mask) && mask[index] {
			selected = append(selected, math.Abs(value))
		}
	}
	if len(selected) == 0 {
		return 0
	}
	sort.Float64s(selected)
	index := int(math.Ceil(clamp(quantile, 0, 1)*float64(len(selected)))) - 1
	return selected[max(0, min(len(selected)-1, index))]
}

func validateSequencePitch(model *SequencePitchModel) error {
	if model == nil {
		return fmt.Errorf("missing sequence_pitch")
	}
	hidden := len(model.InputBias)
	if hidden == 0 || len(model.InputWeights) != hidden || len(model.OutputWeight) != hidden {
		return fmt.Errorf("inconsistent hidden size")
	}
	for _, row := range model.InputWeights {
		if len(row) != len(model.FeatureNames) {
			return fmt.Errorf("input weight width is %d, want %d", len(row), len(model.FeatureNames))
		}
	}
	for _, layer := range model.Layers {
		if layer.Dilation <= 0 || len(layer.Weights) != hidden || len(layer.Bias) != hidden {
			return fmt.Errorf("invalid temporal layer dimensions")
		}
		for _, output := range layer.Weights {
			if len(output) != hidden {
				return fmt.Errorf("invalid temporal layer input size")
			}
			for _, kernel := range output {
				if len(kernel) != 3 {
					return fmt.Errorf("temporal kernel width is %d, want 3", len(kernel))
				}
			}
		}
	}
	if model.Low <= 0 || model.High < model.Low {
		return fmt.Errorf("invalid output bounds %.4f..%.4f", model.Low, model.High)
	}
	return nil
}

func (m *SequencePitchModel) predict(morae []frontend.Mora, frames []FeatureFrame) []float64 {
	result := make([]float64, len(morae))
	for i := range result {
		result[i] = 1
	}
	if len(morae) == 0 || validateSequencePitch(m) != nil {
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
	for _, layer := range m.Layers {
		next := make([][]float64, len(state))
		for position := range state {
			next[position] = make([]float64, hidden)
			for output := 0; output < hidden; output++ {
				value := state[position][output] + layer.Bias[output]
				for input := 0; input < hidden; input++ {
					for kernel := 0; kernel < 3; kernel++ {
						source := position + (kernel-1)*layer.Dilation
						if source >= 0 && source < len(state) {
							value += layer.Weights[output][input][kernel] * state[source][input]
						}
					}
				}
				next[position][output] = math.Tanh(value)
			}
		}
		state = next
	}
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

func featuresFor(morae []frontend.Mora, position int) map[string]float64 {
	current := morae[position]
	denominator := float64(max(1, len(morae)-1))
	pos := float64(position) / denominator
	result := map[string]float64{
		"bias": 1, "position": pos, "position2": pos * pos,
		"from_end": 1 - pos,
	}
	if position == 0 || morae[position-1].Pause {
		result["phrase_start"] = 1
	}
	if position == len(morae)-1 || morae[position+1].Pause {
		result["phrase_end"] = 1
	}
	addCategorical(result, "mora", current)
	if position > 0 {
		addCategorical(result, "prev", morae[position-1])
	} else {
		result["prev=<BOS>"] = 1
	}
	if position+1 < len(morae) {
		addCategorical(result, "next", morae[position+1])
	} else {
		result["next=<EOS>"] = 1
	}
	return result
}

// indexedFeatureVectorsは静的モーラ特徴を一度だけ変換し、フレームごとのmap生成を避ける。
type indexedFeature struct {
	column int
	value  float64
}

func indexedFeatureVectors(morae []frontend.Mora, frames []FeatureFrame, index map[string]int) [][]indexedFeature {
	result := make([][]indexedFeature, len(morae))
	for position := range morae {
		features := featuresFor(morae, position)
		if position < len(frames) {
			for name, value := range frames[position] {
				features[name] = value
			}
		}
		row := make([]indexedFeature, 0, len(features))
		for name, value := range features {
			if column, ok := index[name]; ok {
				if value != 0 {
					row = append(row, indexedFeature{column: column, value: value})
				}
			}
		}
		result[position] = row
	}
	return result
}

func addIndexedFeature(state []float64, weights [][]float64, index map[string]int, name string, value float64) {
	column, ok := index[name]
	if !ok || value == 0 {
		return
	}
	for output := range state {
		state[output] += weights[output][column] * value
	}
}

func addCategorical(features map[string]float64, prefix string, mora frontend.Mora) {
	if mora.Pause {
		features[prefix+"=<PAUSE>"] = 1
		return
	}
	features[prefix+"="+mora.Text] = 1
	features[prefix+"_vowel="+mora.Vowel] = 1
}

func dot(weights, features map[string]float64) float64 {
	result := 0.0
	for _, name := range sortedFeatureNames(features) {
		result += weights[name] * features[name]
	}
	return result
}

func sortedFeatureNames(features map[string]float64) []string {
	names := make([]string, 0, len(features))
	for name := range features {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func clamp(value, low, high float64) float64 {
	return math.Max(low, math.Min(high, value))
}
