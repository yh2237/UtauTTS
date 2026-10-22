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
	Renderer           string                     `json:"renderer"`
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
	RendererSettings   map[string]json.RawMessage `json:"renderer_settings,omitempty"`
	editingRequest
	compatibilityRequest
}

type editingRequest struct {
	MoraDurationsMS []float64                `json:"mora_durations_ms"`
	UnitOverrides   []plan.UnitOverride      `json:"unit_overrides"`
	ManualPitch     *prosody.ManualPitchFile `json:"manual_pitch"`
	PitchCurve      *render.PitchCurve       `json:"pitch_curve,omitempty"`
	// 廃止したClassic UTAU用。受け付けて無視する。
	ResamplerExpressions json.RawMessage `json:"resampler_expressions"`
}

// 無視する旧フィールドも受け付け、型付きProvider設定の優先順位を保つ。
type compatibilityRequest struct {
	Kana                    string                          `json:"kana"`
	SpeechTiming            bool                            `json:"speech_timing"`
	ModelPath               string                          `json:"model_path"`
	Resampler               string                          `json:"resampler"`
	Wavtool                 string                          `json:"wavtool"`
	AliasPolicy             voicebank.AliasPolicy           `json:"alias_policy"`
	WordBoundaryEnvelope    bool                            `json:"word_boundary_envelope"`
	SpeechProsodyExperiment string                          `json:"prosody_experiment"`
	LeadingPreutteranceMS   float64                         `json:"leading_preutterance_ms"`
	ReleaseMS               float64                         `json:"release_ms"`
	ReleaseSet              bool                            `json:"release_set"`
	ContextDurationStrength float64                         `json:"context_duration_strength"`
	BoundaryToneStrength    float64                         `json:"boundary_tone_strength"`
	StretchAdaptStrength    float64                         `json:"stretch_adapt_strength"`
	PauseContextStrength    float64                         `json:"pause_context_strength"`
	DiffSingerSteps         int64                           `json:"diffsinger_steps"`
	DiffSingerDurationMix   float64                         `json:"diffsinger_duration_mix"`
	DiffSingerPitchMix      float64                         `json:"diffsinger_pitch_mix"`
	DiffSingerExpr          float64                         `json:"diffsinger_expr"`
	Worldline               render.WorldlineProviderOptions `json:"worldline,omitempty"`
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
		VoicebankID: request.VoicebankID, RendererID: request.Renderer, ModelID: request.ModelID,
		Tone: request.Tone, Color: request.Color, Dictionary: request.Dictionary,
		Method:           request.Method,
		RendererSettings: request.RendererSettings,
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
	input.DiffSingerSteps, input.DiffSingerExpr = request.DiffSingerSteps, request.DiffSingerExpr
	input.DiffSingerDurationMix, input.DiffSingerPitchMix = request.DiffSingerDurationMix, request.DiffSingerPitchMix
	input.Worldline = request.Worldline
	return input
}
