package prosody

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"utautts/internal/frontend"
)

// SpeechModelは自然音声で学習する残差モデル。未知音素は規則を使う。
type SpeechModel struct {
	Version          int                  `json:"version"`
	FeatureVersion   int                  `json:"feature_version"`
	ID               string               `json:"id"`
	Language         string               `json:"language"`
	Corpus           string               `json:"corpus"`
	License          string               `json:"license"`
	TrainingDataKind string               `json:"training_data_kind"`
	PhoneCounts      map[string]int       `json:"phone_counts"`
	PitchPhoneCounts map[string]int       `json:"pitch_phone_counts,omitempty"`
	Duration         map[string]float64   `json:"duration_log_ratio"`
	Energy           map[string]float64   `json:"energy_log_ratio,omitempty"`
	Pitch            []map[string]float64 `json:"pitch_cents,omitempty"`
}

func LoadSpeechModel(path string) (*SpeechModel, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var model SpeechModel
	if err := json.Unmarshal(data, &model); err != nil {
		return nil, err
	}
	if err := model.Validate(); err != nil {
		return nil, err
	}
	return &model, nil
}

func (m *SpeechModel) Validate() error {
	if m == nil || m.Version != 1 || m.FeatureVersion != 1 || m.ID == "" || (m.Language != "en" && m.Language != "zh") {
		return fmt.Errorf("unsupported speech model identity/version/language")
	}
	if m.Corpus == "" || m.License == "" || m.TrainingDataKind != "natural" {
		return fmt.Errorf("speech model requires aligned natural-speech provenance")
	}
	if len(m.PhoneCounts) == 0 || len(m.Duration) == 0 {
		return fmt.Errorf("empty speech model coverage or duration head")
	}
	for phone, count := range m.PhoneCounts {
		if phone == "" || count < 1 {
			return fmt.Errorf("invalid speech model phone coverage")
		}
	}
	for phone, count := range m.PitchPhoneCounts {
		if count < 1 || count > m.PhoneCounts[phone] {
			return fmt.Errorf("invalid pitch phone coverage")
		}
	}
	if len(m.Pitch) != 0 && len(m.Pitch) != 3 {
		return fmt.Errorf("speech model pitch head needs 3 knots")
	}
	if len(m.Pitch) > 0 && len(m.PitchPhoneCounts) == 0 {
		return fmt.Errorf("pitch head requires voiced-phone coverage")
	}
	heads := []map[string]float64{m.Duration, m.Energy}
	heads = append(heads, m.Pitch...)
	for _, head := range heads {
		for key, value := range head {
			if key == "" || math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > 1e6 {
				return fmt.Errorf("invalid speech model coefficient")
			}
		}
	}
	return nil
}

func (m *SpeechModel) Covers(phone frontend.Phone) bool {
	return m != nil && m.PhoneCounts[strings.ToLower(phone.Symbol)] >= 5
}

func SpeechLinear(head map[string]float64, features []string) float64 {
	value := 0.0
	for _, feature := range features {
		value += head[feature]
	}
	return value
}

// SpeechPhoneFeaturesは学習と推論で共通の音素特徴を返す。
func SpeechPhoneFeatures(morae []frontend.Mora) [][][]string {
	result := make([][][]string, len(morae))
	type phoneRef struct{ position, phone int }
	var phrase []phoneRef
	flush := func() {
		for i, ref := range phrase {
			m := morae[ref.position]
			p := m.Phones[ref.phone]
			prev, next := "#", "#"
			if i > 0 {
				r := phrase[i-1]
				prev = strings.ToLower(morae[r.position].Phones[r.phone].Symbol)
			}
			if i+1 < len(phrase) {
				r := phrase[i+1]
				next = strings.ToLower(morae[r.position].Phones[r.phone].Symbol)
			}
			prevTone, nextTone := "#", "#"
			for k := i - 1; k >= 0; k-- {
				if phrase[k].position != ref.position {
					prevTone = strconv.Itoa(morae[phrase[k].position].Tone)
					break
				}
			}
			for k := i + 1; k < len(phrase); k++ {
				if phrase[k].position != ref.position {
					nextTone = strconv.Itoa(morae[phrase[k].position].Tone)
					break
				}
			}
			stress := "unknown"
			if m.StressKnown && p.Role == "nucleus" {
				stress = strconv.Itoa(m.Stress)
			}
			features := []string{"bias", "phone=" + strings.ToLower(p.Symbol), "role=" + p.Role, "prev=" + prev, "next=" + next,
				"stress=" + stress, "tone=" + strconv.Itoa(m.Tone)}
			if m.Language == frontend.LanguageChinese {
				features = append(features, "prev_tone="+prevTone, "next_tone="+nextTone)
			}
			if i == 0 {
				features = append(features, "phrase_initial")
			}
			if i == len(phrase)-1 {
				features = append(features, "phrase_final")
			}
			if i+1 == len(phrase) || morae[phrase[i+1].position].WordIndex != m.WordIndex {
				features = append(features, "word_final")
			}
			result[ref.position][ref.phone] = features
		}
		phrase = nil
	}
	for i, m := range morae {
		result[i] = make([][]string, len(m.Phones))
		if m.Pause {
			flush()
			continue
		}
		for j := range m.Phones {
			phrase = append(phrase, phoneRef{i, j})
		}
	}
	flush()
	return result
}
