package entryinput

import (
	"bytes"
	"encoding/json"

	utautts "github.com/yh2237/UtauTTS"
	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/prosody"
	"github.com/yh2237/UtauTTS/internal/render"
	"github.com/yh2237/UtauTTS/internal/synth"
	"github.com/yh2237/UtauTTS/internal/voicebank"
)

// JSONRequestはCLI・HTTPの入力形式。編集・互換フィールドも同じ階層で受け取る。
type JSONRequest struct {
	Text               string                     `json:"text"`
	Reading            string                     `json:"reading"`
	Language           string                     `json:"language"`
	Phonemizer         string                     `json:"phonemizer"`
	VoicebankID        string                     `json:"voicebank_id"`
	ModelID            string                     `json:"model_id"`
	Tone               string                     `json:"tone"`
	Color              string                     `json:"color"`
	Method             string                     `json:"method"`
	Dictionary         []utautts.DictionaryEntry  `json:"dictionary"`
	MoraDurationMS     float64                    `json:"mora_duration_ms"`
	PauseDurationMS    float64                    `json:"pause_duration_ms"`
	IntonationStrength float64                    `json:"intonation_strength"`
	ApplyPitch         bool                       `json:"apply_pitch"`
	ContextDuration    bool                       `json:"context_duration"`
	BoundaryTone       bool                       `json:"boundary_tone"`
	StretchAdapt       bool                       `json:"stretch_adapt"`
	PauseContext       bool                       `json:"pause_context"`
	Settings           map[string]json.RawMessage `json:"settings,omitempty"`
	editingRequest
	compatibilityRequest
}

type editingRequest struct {
	MoraDurationsMS []float64                `json:"mora_durations_ms"`
	UnitOverrides   []plan.UnitOverride      `json:"unit_overrides"`
	ManualPitch     *prosody.ManualPitchFile `json:"manual_pitch"`
	PitchCurve      *render.PitchCurve       `json:"pitch_curve,omitempty"`
}

// 互換用の入力。settingsにも同じIDがあればそちらを優先する。
type compatibilityRequest struct {
	Kana                    string                `json:"kana"`
	ModelPath               string                `json:"model_path"`
	AliasPolicy             voicebank.AliasPolicy `json:"alias_policy"`
	LeadingPreutteranceMS   float64               `json:"leading_preutterance_ms"`
	ReleaseMS               float64               `json:"release_ms"`
	ReleaseSet              bool                  `json:"release_set"`
	ContextDurationStrength float64               `json:"context_duration_strength"`
	BoundaryToneStrength    float64               `json:"boundary_tone_strength"`
	StretchAdaptStrength    float64               `json:"stretch_adapt_strength"`
	PauseContextStrength    float64               `json:"pause_context_strength"`
}

type jsonRequestAlias JSONRequest

func defaultJSONRequest() JSONRequest {
	defaults := synth.DefaultRequest()
	return JSONRequest{
		MoraDurationMS: defaults.MoraDurationMS, PauseDurationMS: defaults.PauseDurationMS,
		IntonationStrength: defaults.IntonationStrength, ApplyPitch: defaults.ApplyPitch,
		ContextDuration: defaults.ContextDuration, BoundaryTone: defaults.BoundaryTone,
		StretchAdapt: defaults.StretchAdapt, PauseContext: defaults.PauseContext,
		compatibilityRequest: compatibilityRequest{
			ContextDurationStrength: defaults.ContextDurationStrength,
			BoundaryToneStrength:    defaults.BoundaryToneStrength,
			StretchAdaptStrength:    defaults.StretchAdaptStrength,
			PauseContextStrength:    defaults.PauseContextStrength,
		},
	}
}

func (request *JSONRequest) UnmarshalJSON(data []byte) error {
	decoded := jsonRequestAlias(defaultJSONRequest())
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	*request = JSONRequest(decoded)
	return nil
}

// DecodeRequestLenientは共通既定値を適用し、未知のフィールドを無視する。
// 同梱UIのnative/C ABI向け。CLI・HTTPは厳密に検証する。
func DecodeRequestLenient(data []byte, request *JSONRequest) error {
	*request = defaultJSONRequest()
	return json.Unmarshal(data, (*jsonRequestAlias)(request))
}

func (request JSONRequest) ReadingOrKana() string {
	if request.Reading != "" {
		return request.Reading
	}
	return request.Kana
}

func (request JSONRequest) publicRequest() utautts.Request {
	return utautts.Request{
		Text: request.Text, Reading: request.ReadingOrKana(),
		Language: request.Language, Phonemizer: request.Phonemizer,
		VoicebankID: request.VoicebankID, ModelID: request.ModelID,
		Tone: request.Tone, Color: request.Color, Dictionary: request.Dictionary,
		Method:   request.Method,
		Settings: request.Settings,
		Options: utautts.Options{
			MoraDurationMS: &request.MoraDurationMS, PauseDurationMS: &request.PauseDurationMS,
			IntonationStrength: &request.IntonationStrength, ApplyPitch: &request.ApplyPitch,
			ContextDuration: &request.ContextDuration, BoundaryTone: &request.BoundaryTone,
			StretchAdapt: &request.StretchAdapt, PauseContext: &request.PauseContext,
		},
	}
}

func (request JSONRequest) InternalRequest() synth.Request {
	input := Apply(request.publicRequest(), synth.DefaultRequest())
	input.MoraDurationsMS, input.UnitOverrides = request.MoraDurationsMS, request.UnitOverrides
	input.ManualPitch, input.PitchCurve = request.ManualPitch, request.PitchCurve
	input.ModelPath = request.ModelPath
	input.AliasPolicy = request.AliasPolicy
	input.LeadingPreutteranceMS = request.LeadingPreutteranceMS
	input.ReleaseMS, input.ReleaseSet = request.ReleaseMS, request.ReleaseSet
	input.ContextDurationStrength = request.ContextDurationStrength
	input.BoundaryToneStrength = request.BoundaryToneStrength
	input.StretchAdaptStrength = request.StretchAdaptStrength
	input.PauseContextStrength = request.PauseContextStrength
	return input
}
