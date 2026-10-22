package prosody

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/speechtiming"
)

func LoadModel(path string) (*Model, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var head f0HeadManifest
	if err := json.Unmarshal(data, &head); err == nil && (head.BaseModel != "" || len(head.BaseModelData) > 0) {
		return loadF0HeadModel(path, data, head)
	}
	return ParseModel(data)
}

// f0HeadManifestは、基準の抑揚モデルにF0ヘッド（統合韻律モデル）を重ねるモデルJSON。
// 基準モデルがアクセント特徴とモーラの予測を、F0ヘッドが自動ピッチ曲線とモーラの音量を担う。
type f0HeadManifest struct {
	// BaseModelは同じディレクトリにある基準モデルのJSON。BaseModelDataがあればそちらを使う。
	BaseModel string `json:"base_model"`
	// BaseModelDataは埋め込んだ基準モデルのJSON（基準モデルを別に配布しない場合）。
	BaseModelData json.RawMessage `json:"base_model_data"`
	// F0HeadはF0（とエネルギー）ヘッドを持つ時間伸縮系モデルのsafetensors（JSONではbase64）。
	F0Head []byte `json:"f0_head"`
	// UseEnergyはエネルギーヘッドでモーラの音量を変えるか。未指定は使う。
	UseEnergy *bool `json:"use_energy"`
	// BaseBlendは基準モデルの抑揚曲線を混ぜる重み（0〜1）。0はF0ヘッドだけを使う。
	BaseBlend float64 `json:"base_blend"`
	// PitchOffsetCentsはF0ヘッドの自動ピッチ曲線全体へ足す高さ（セント）。
	PitchOffsetCents float64 `json:"pitch_offset_cents"`
}

func loadF0HeadModel(path string, data []byte, head f0HeadManifest) (*Model, error) {
	var base *Model
	var err error
	if len(head.BaseModelData) > 0 {
		base, err = ParseModel(head.BaseModelData)
		if err != nil {
			return nil, fmt.Errorf("parse embedded base model: %w", err)
		}
	} else {
		base, err = LoadModel(filepath.Join(filepath.Dir(path), filepath.FromSlash(head.BaseModel)))
		if err != nil {
			return nil, fmt.Errorf("load base model %q: %w", head.BaseModel, err)
		}
	}
	if base.F0Head != nil {
		return nil, fmt.Errorf("base model must not have an F0 head")
	}
	tcn, err := speechtiming.LoadTCN(head.F0Head)
	if err != nil {
		return nil, fmt.Errorf("load F0 head: %w", err)
	}
	if !tcn.HasF0Head() {
		return nil, fmt.Errorf("F0 head model has no F0 head")
	}
	var identity struct {
		ID              string           `json:"id"`
		DisplayName     string           `json:"display_name"`
		Description     string           `json:"description"`
		License         string           `json:"license"`
		LicenseNotices  []string         `json:"license_notices"`
		Language        string           `json:"language"`
		Provenance      *ModelProvenance `json:"provenance"`
		DefaultPriority int              `json:"default_priority"`
	}
	if err := json.Unmarshal(data, &identity); err != nil {
		return nil, err
	}
	model := *base
	model.ID, model.DisplayName, model.Description = identity.ID, identity.DisplayName, identity.Description
	model.License, model.LicenseNotices, model.Provenance = identity.License, identity.LicenseNotices, identity.Provenance
	model.Language, model.DefaultPriority = identity.Language, identity.DefaultPriority
	model.F0Head = tcn
	model.F0HeadEnergy = head.UseEnergy == nil || *head.UseEnergy
	if head.BaseBlend < 0 || head.BaseBlend > 1 {
		return nil, fmt.Errorf("base_blend must be between 0 and 1, got %v", head.BaseBlend)
	}
	model.F0HeadBaseBlend = head.BaseBlend
	if math.IsNaN(head.PitchOffsetCents) || math.Abs(head.PitchOffsetCents) > 1200 {
		return nil, fmt.Errorf("pitch_offset_cents must be between -1200 and 1200, got %v", head.PitchOffsetCents)
	}
	model.F0HeadPitchOffsetCents = head.PitchOffsetCents
	return &model, nil
}

func ParseModel(data []byte) (*Model, error) {
	var model Model
	if err := json.Unmarshal(data, &model); err != nil {
		return nil, err
	}
	frame := model.FeatureVersion == 1 && model.Version == FramePitchModelVersion && model.Mode == "intonation_frame_tcn_accent_bounded"
	if !frame {
		return nil, fmt.Errorf("unsupported prosody model version %d/feature %d mode %q", model.Version, model.FeatureVersion, model.Mode)
	}
	allowedHeads := []string{"frame_pitch"}
	for _, head := range model.heads() {
		if head.present && !containsString(allowedHeads, head.name) {
			return nil, fmt.Errorf("prosody model head %q is not supported by mode %q", head.name, model.Mode)
		}
	}
	if err := validateFramePitch(model.FramePitch); err != nil {
		return nil, fmt.Errorf("invalid frame pitch model: %w", err)
	}
	model.FramePitch.validated = true
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
		{"frame_pitch", m.FramePitch != nil},
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

func validateFramePitch(model *FramePitchModel) error {
	if model == nil || model.FrameMS < 1 || model.LowCents >= model.HighCents || model.LowCents > 0 || model.HighCents < 0 {
		return fmt.Errorf("invalid frame pitch metadata")
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
	return nil
}
