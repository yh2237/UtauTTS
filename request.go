package utautts

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/yh2237/UtauTTS/internal/settings"
	"github.com/yh2237/UtauTTS/internal/synth"
)

func (r Request) internal() (synth.Request, error) {
	if strings.TrimSpace(r.Text) == "" && strings.TrimSpace(r.Reading) == "" {
		return synth.Request{}, fmt.Errorf("%w: text or reading is required", ErrInvalidRequest)
	}
	input := synth.ApplyCommonOptions(synth.DefaultRequest(), synth.CommonOptions(r.Options))
	input.Text, input.Reading, input.Language, input.Phonemizer = r.Text, r.Reading, r.Language, r.Phonemizer
	input.VoicebankID, input.ModelID = r.VoicebankID, r.ModelID
	input.Tone, input.Color = r.Tone, r.Color
	input.Method = r.Method
	for _, entry := range r.Dictionary {
		if entry.Surface == "" || entry.Reading == "" {
			return synth.Request{}, fmt.Errorf("%w: dictionary surface and reading are required", ErrInvalidRequest)
		}
		input.Dictionary = append(input.Dictionary, synth.DictionaryEntry{Surface: entry.Surface, Reading: entry.Reading})
	}
	numbers := []struct {
		id    string
		value *float64
	}{
		{"mora_duration_ms", r.Options.MoraDurationMS},
		{"pause_duration_ms", r.Options.PauseDurationMS},
		{"intonation_strength", r.Options.IntonationStrength},
	}
	for _, option := range numbers {
		if option.value == nil {
			continue
		}
		setting, _ := settings.Lookup(option.id)
		if err := validateNumber(option.id, *option.value, setting.Min, setting.Max); err != nil {
			return synth.Request{}, err
		}
	}
	if r.Options.ReleaseMS != nil {
		if err := validateNumber("release_ms", *r.Options.ReleaseMS, new(0.0), nil); err != nil {
			return synth.Request{}, err
		}
	}
	input.Settings = make(map[string]json.RawMessage, len(r.Settings))
	for id, raw := range r.Settings {
		switch id {
		case "mora_duration_ms", "pause_duration_ms", "intonation_strength", "release_ms", "apply_pitch",
			"context_duration", "boundary_tone", "stretch_adapt", "pause_context":
			return synth.Request{}, fmt.Errorf("%w: %s belongs in Options", ErrInvalidRequest, id)
		}
		if !json.Valid(raw) || strings.TrimSpace(string(raw)) == "null" {
			return synth.Request{}, fmt.Errorf("%w: setting %q must contain a non-null JSON value", ErrInvalidRequest, id)
		}
		setting, known := settings.Lookup(id)
		if !known {
			return synth.Request{}, fmt.Errorf("%w: unknown setting %q", ErrInvalidRequest, id)
		}
		switch setting.Type {
		case "number", "integer":
			var value float64
			if err := json.Unmarshal(raw, &value); err != nil || (setting.Type == "integer" && value != math.Trunc(value)) {
				return synth.Request{}, fmt.Errorf("%w: setting %q expects %s", ErrInvalidRequest, id, setting.Type)
			}
			if err := validateNumber(id, value, setting.Min, setting.Max); err != nil {
				return synth.Request{}, err
			}
		case "boolean":
			var value bool
			if err := json.Unmarshal(raw, &value); err != nil {
				return synth.Request{}, fmt.Errorf("%w: setting %q expects boolean", ErrInvalidRequest, id)
			}
		case "string", "enum":
			var value string
			if err := json.Unmarshal(raw, &value); err != nil {
				return synth.Request{}, fmt.Errorf("%w: setting %q expects string", ErrInvalidRequest, id)
			}
		}
		input.Settings[id] = append(json.RawMessage(nil), raw...)
	}
	return input, nil
}

func validateNumber(id string, value float64, min, max *float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) || (min != nil && value < *min) || (max != nil && value > *max) {
		return fmt.Errorf("%w: %s is outside the supported range", ErrInvalidRequest, id)
	}
	return nil
}
