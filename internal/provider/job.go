package provider

import (
	"encoding/json"

	"utautts/internal/engine"
)

const (
	// CapabilityUnitRendererJobV2は共通unit renderer入力を示す。
	CapabilityUnitRendererJobV2 = "unit_renderer_job_v2"
	// CapabilityNeuralScoreJobV1は共通ニューラル入力を示す。
	CapabilityNeuralScoreJobV1 = "neural_score_job_v1"
)

// UnitRendererJobVersionはunit renderer入力の版を示す。
const UnitRendererJobVersion = 2

// UnitRendererJobはunit rendererへ渡す共通入力を示す。
type UnitRendererJob struct {
	Version         int                 `json:"version"`
	Contract        string              `json:"contract"`
	ContractVersion int                 `json:"contract_version"`
	Plan            json.RawMessage     `json:"plan"`
	Options         UnitRendererOptions `json:"options"`
	Resources       map[string]string   `json:"resources,omitempty"`
}

// UnitRendererOptionsはrenderer間で共有する設定を示す。
type UnitRendererOptions struct {
	ReleaseMS               float64           `json:"release_ms"`
	LeadingPreutteranceMS   float64           `json:"leading_preutterance_ms"`
	IntonationStrength      float64           `json:"intonation_strength"`
	ApplyPitch              bool              `json:"apply_pitch"`
	BoundaryBridgeMS        float64           `json:"boundary_bridge_ms"`
	BoundaryBridgeThreshold float64           `json:"boundary_bridge_threshold"`
	CVVCTiming              string            `json:"cvvc_timing,omitempty"`
	CVVCTransitionGain      float64           `json:"cvvc_transition_gain,omitempty"`
	CVVCPreBoundaryFade     bool              `json:"cvvc_pre_boundary_fade,omitempty"`
	PitchCurve              *PitchCurve       `json:"pitch_curve,omitempty"`
	Worldline               *WorldlineOptions `json:"worldline,omitempty"`
}

// WorldlineOptionsはWORLD固有の入力を示す。
type WorldlineOptions struct {
	Engine      string          `json:"engine"`
	SampleRate  int             `json:"sample_rate"`
	ExactLength bool            `json:"exact_length,omitempty"`
	F0Curve     []float64       `json:"f0_curve"`
	Units       []WorldlineUnit `json:"units"`
}

type WorldlineUnit struct {
	Speech            *WorldSpeechTiming       `json:"speech,omitempty"`
	LegacyMix         bool                     `json:"legacy_mix,omitempty"`
	GapRepair         bool                     `json:"gap_repair,omitempty"`
	CacheKey          string                   `json:"cache_key,omitempty"`
	Source            string                   `json:"source"`
	FRQPath           string                   `json:"frq_path,omitempty"`
	PositionMS        float64                  `json:"position_ms"`
	SkipMS            float64                  `json:"skip_ms"`
	LengthMS          float64                  `json:"length_ms"`
	FadeInMS          float64                  `json:"fade_in_ms"`
	FadeOutMS         float64                  `json:"fade_out_ms"`
	OffsetMS          float64                  `json:"offset_ms"`
	RequiredLengthMS  float64                  `json:"required_length_ms"`
	ConsonantMS       float64                  `json:"consonant_ms"`
	CutoffMS          float64                  `json:"cutoff_ms"`
	Tone              int                      `json:"tone"`
	ConsonantVelocity float64                  `json:"consonant_velocity"`
	PitchStartMS      float64                  `json:"pitch_start_ms,omitempty"`
	PitchLengthMS     float64                  `json:"pitch_length_ms,omitempty"`
	Volume            float64                  `json:"volume,omitempty"`
	VolumeSet         bool                     `json:"volume_set,omitempty"`
	Modulation        float64                  `json:"modulation,omitempty"`
	Tempo             float64                  `json:"tempo,omitempty"`
	EnergyFactor      float64                  `json:"energy_factor,omitempty"`
	Envelope          []WorldlineEnvelopePoint `json:"envelope,omitempty"`
}

const CapabilityWorldSpeechV1 = "world_speech_v1"
const CapabilityCodaReleaseV1 = "coda_release_v1"

// WorldSpeechTimingは切り出し後の音源を基準とする位置を示す。
type WorldSpeechTiming struct {
	CodaRelease               bool    `json:"coda_release,omitempty"`
	PreserveStopOnly          bool    `json:"preserve_stop_only,omitempty"`
	UnitIndex                 int     `json:"unit_index"`
	SourceOnsetMS             float64 `json:"source_onset_ms"`
	SourceTransientMS         float64 `json:"source_transient_ms,omitempty"`
	SourceTransientDurationMS float64 `json:"source_transient_duration_ms,omitempty"`
	TargetOnsetMS             float64 `json:"target_onset_ms"`
	ProtectStop               bool    `json:"protect_stop,omitempty"`
	VowelJoin                 bool    `json:"vowel_join,omitempty"`
	TargetFixedMS             float64 `json:"target_fixed_ms,omitempty"`
	TargetJoinMS              float64 `json:"target_join_ms,omitempty"`
	TransitionLeftPhone       string  `json:"transition_left_phone,omitempty"`
	TransitionRightPhone      string  `json:"transition_right_phone,omitempty"`
}

type WorldSpeechResult struct {
	UnitIndex         int     `json:"unit_index"`
	RetimeApplied     bool    `json:"retime_applied"`
	TargetFixedMS     float64 `json:"target_fixed_ms"`
	JoinApplied       bool    `json:"join_applied"`
	TransitionApplied bool    `json:"transition_applied,omitempty"`
	StopBurstApplied  bool    `json:"stop_burst_applied,omitempty"`
	StopBurstGain     float64 `json:"stop_burst_gain,omitempty"`
}

type WorldlineEnvelopePoint struct {
	XMS float64 `json:"x_ms"`
	Y   float64 `json:"y"`
}

// PitchCurveはフレーム単位のピッチ曲線を示す。
type PitchCurve struct {
	FrameMS float64   `json:"frame_ms"`
	Cents   []float64 `json:"cents"`
}

// NeuralSynthesizerJobVersionは共通ニューラル入力の版を示す。
const NeuralSynthesizerJobVersion = 1

// NeuralSynthesizerJobはニューラル音声合成へ渡す入力を示す。
type NeuralSynthesizerJob struct {
	Version         int                `json:"version"`
	Contract        string             `json:"contract"`
	ContractVersion int                `json:"contract_version"`
	Score           engine.NeuralScore `json:"score"`
	Options         json.RawMessage    `json:"options,omitempty"`
	Resources       map[string]string  `json:"resources,omitempty"`
}
