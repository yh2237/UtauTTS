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

// JSONRequest is the CLI/HTTP wire format, independent of evaluation-only
// fields in synth.Request. Embedded fields keep the existing flat JSON format.
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
	Dictionary         []utautts.DictionaryEntry  `json:"dictionary"`
	MoraDurationMS     float64                    `json:"mora_duration_ms"`
	PauseDurationMS    float64                    `json:"pause_duration_ms"`
	IntonationStrength float64                    `json:"intonation_strength"`
	ApplyPitch         bool                       `json:"apply_pitch"`
	ContextDuration    bool                       `json:"context_duration"`
	BoundaryTone       bool                       `json:"boundary_tone"`
	StretchAdapt       bool                       `json:"stretch_adapt"`
	PauseContext       bool                       `json:"pause_context"`
	EnglishWeakForm    bool                       `json:"english_weak_form"`
	RendererSettings   map[string]json.RawMessage `json:"renderer_settings,omitempty"`
	editingRequest
	compatibilityRequest
}

type editingRequest struct {
	MoraDurationsMS      []float64                    `json:"mora_durations_ms"`
	UnitOverrides        []plan.UnitOverride          `json:"unit_overrides"`
	ManualPitch          *prosody.ManualPitchFile     `json:"manual_pitch"`
	PitchCurve           *render.PitchCurve           `json:"pitch_curve,omitempty"`
	ResamplerExpressions []render.ResamplerExpression `json:"resampler_expressions"`
}

// Compatibility fields are not part of the normal public utterance. Ignored
// legacy fields remain accepted; typed provider settings keep their priority.
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

// jsonRequestAlias avoids recursive UnmarshalJSON calls.
type jsonRequestAlias JSONRequest

func defaultJSONRequest() JSONRequest {
	defaults := synth.DefaultRequest()
	return JSONRequest{
		MoraDurationMS: defaults.MoraDurationMS, PauseDurationMS: defaults.PauseDurationMS,
		IntonationStrength: defaults.IntonationStrength, ApplyPitch: defaults.ApplyPitch,
		ContextDuration: defaults.ContextDuration, BoundaryTone: defaults.BoundaryTone,
		StretchAdapt: defaults.StretchAdapt, PauseContext: defaults.PauseContext,
		EnglishWeakForm: defaults.EnglishWeakForm,
		compatibilityRequest: compatibilityRequest{
			Wavtool:                 defaults.Wavtool,
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

// DecodeRequestLenient applies the shared defaults and ignores fields that are
// not part of the shared request. It is for a trusted local UI entry such as
// the native/C ABI, where the UI and the library ship together. The CLI and
// HTTP keep the strict UnmarshalJSON validation.
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
		RendererSettings: request.RendererSettings,
		Options: utautts.Options{
			MoraDurationMS: &request.MoraDurationMS, PauseDurationMS: &request.PauseDurationMS,
			IntonationStrength: &request.IntonationStrength, ApplyPitch: &request.ApplyPitch,
			ContextDuration: &request.ContextDuration, BoundaryTone: &request.BoundaryTone,
			StretchAdapt: &request.StretchAdapt, PauseContext: &request.PauseContext,
			EnglishWeakForm: &request.EnglishWeakForm,
		},
	}
}

func (request JSONRequest) InternalRequest() synth.Request {
	input := Apply(request.publicRequest(), synth.DefaultRequest())
	input.MoraDurationsMS, input.UnitOverrides = request.MoraDurationsMS, request.UnitOverrides
	input.ManualPitch, input.PitchCurve = request.ManualPitch, request.PitchCurve
	input.ResamplerExpressions = request.ResamplerExpressions
	input.ModelPath = request.ModelPath
	input.Resampler, input.Wavtool, input.AliasPolicy = request.Resampler, request.Wavtool, request.AliasPolicy
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
