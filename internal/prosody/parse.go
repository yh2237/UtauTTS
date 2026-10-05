package prosody

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"utautts/internal/frontend"
)

func LoadModel(path string) (*Model, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseModel(data)
}

func ParseModel(data []byte) (*Model, error) {
	var model Model
	if err := json.Unmarshal(data, &model); err != nil {
		return nil, err
	}
	current := model.Version == ModelVersion && model.FeatureVersion == 1 && model.Mode == "speech_prosody_residual"
	frame := model.FeatureVersion == 1 && model.Version == FramePitchModelVersion && (model.Mode == "intonation_frame_tcn_accent_bounded" || model.Mode == "intonation_frame_tcn_english_bounded")
	if model.Mode == "intonation_frame_tcn_english_bounded" && model.Language != "en" {
		return nil, fmt.Errorf("English frame model must declare language en")
	}
	multitask := model.FeatureVersion == 2 && model.Version == ProsodyMultitaskModelVersion && model.Mode == "prosody_multitask_tcn"
	manualResidual := model.FeatureVersion == 2 && model.Version == ManualResidualModelVersion &&
		(model.Mode == "intonation_frame_v8_manual_residual" || model.Mode == "intonation_frame_manual_residual")
	englishIntonation := model.FeatureVersion == 1 && model.Version == EnglishIntonationModelVersion && model.Mode == "english_intonation_v1"
	mandarinIntonation := model.FeatureVersion == 1 && model.Version == MandarinIntonationModelVersion && model.Mode == "mandarin_intonation_v1"
	if !current && !frame && !multitask && !manualResidual && !englishIntonation && !mandarinIntonation {
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
	case mandarinIntonation:
		allowedHeads = []string{"mandarin_intonation"}
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
		model.FramePitch.validated = true
	}
	if multitask {
		if err := validateSequencePitch(model.MoraDuration); err != nil {
			return nil, fmt.Errorf("invalid mora duration model: %w", err)
		}
		model.MoraDuration.validated = true
		if err := validateFramePitch(model.FramePitch); err != nil {
			return nil, fmt.Errorf("invalid multitask frame pitch model: %w", err)
		}
		model.FramePitch.validated = true
	}
	if manualResidual {
		if err := validateFramePitch(model.FramePitch); err != nil {
			return nil, fmt.Errorf("invalid manual residual frame pitch model: %w", err)
		}
		model.FramePitch.validated = true
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
	if mandarinIntonation {
		if model.Language != "zh" || validateMandarinIntonation(model.MandarinIntonation) != nil {
			return nil, fmt.Errorf("invalid Mandarin intonation model")
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
		{"mandarin_intonation", m.MandarinIntonation != nil},
	}
}

// 言語未指定の旧モデルは日本語として扱う。
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
	if model.PhraseFinalLow < 0 || model.PhraseFinalLow > model.High || math.IsNaN(model.PhraseFinalLow) {
		return fmt.Errorf("invalid phrase-final lower bound %.4f", model.PhraseFinalLow)
	}
	if model.PhraseStartHigh != 0 && (model.PhraseStartHigh < model.Low || math.IsNaN(model.PhraseStartHigh)) {
		return fmt.Errorf("invalid phrase-start upper bound %.4f", model.PhraseStartHigh)
	}
	return nil
}
