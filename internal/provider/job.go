package provider

import (
	"encoding/json"

	"utautts/internal/engine"
)

const (
	// CapabilityUnitRendererJobV2 marks the common unit-renderer job envelope
	// without the removed opaque provider payload.
	CapabilityUnitRendererJobV2 = "unit_renderer_job_v2"
	// CapabilityNeuralScoreJobV1 marks the common neural-synthesizer job
	// envelope. It lets older providers fail handshake and use legacy fallback.
	CapabilityNeuralScoreJobV1 = "neural_score_job_v1"
)

// UnitRendererJobVersion is the version of the host-owned unit-renderer job
// file. The transport request only carries paths to this file; the contract
// payload stays language-neutral and can be consumed by a non-Go provider.
const UnitRendererJobVersion = 2

// UnitRendererJob is the common v2 payload for providers that render a
// selected UTAU Unit Plan. Providers consume the logical Plan, typed Options,
// and named Resources; there is no opaque provider payload escape hatch.
type UnitRendererJob struct {
	Version         int                 `json:"version"`
	Contract        string              `json:"contract"`
	ContractVersion int                 `json:"contract_version"`
	Plan            json.RawMessage     `json:"plan"`
	Options         UnitRendererOptions `json:"options"`
	Resources       map[string]string   `json:"resources,omitempty"`
}

// UnitRendererOptions are renderer-independent controls shared by the
// built-in and external unit-renderer adapters.
type UnitRendererOptions struct {
	ReleaseMS               float64     `json:"release_ms"`
	LeadingPreutteranceMS   float64     `json:"leading_preutterance_ms"`
	IntonationStrength      float64     `json:"intonation_strength"`
	ApplyPitch              bool        `json:"apply_pitch"`
	BoundaryBridgeMS        float64     `json:"boundary_bridge_ms"`
	BoundaryBridgeThreshold float64     `json:"boundary_bridge_threshold"`
	CVVCTiming              string      `json:"cvvc_timing,omitempty"`
	CVVCTransitionGain      float64     `json:"cvvc_transition_gain,omitempty"`
	CVVCPreBoundaryFade     bool        `json:"cvvc_pre_boundary_fade,omitempty"`
	PitchCurve              *PitchCurve `json:"pitch_curve,omitempty"`
	// Worldline carries the prepared, typed input required by the bundled
	// WORLD providers. It is part of Options so the bridge consumes the same
	// common job envelope as every other unit renderer.
	Worldline *WorldlineOptions `json:"worldline,omitempty"`
}

// WorldlineOptions is the typed WORLD provider extension of a unit-renderer
// job. It replaces the historical standalone manifest and is deliberately
// nested under the common Options object.
type WorldlineOptions struct {
	Engine      string          `json:"engine"`
	SampleRate  int             `json:"sample_rate"`
	ExactLength bool            `json:"exact_length,omitempty"`
	F0Curve     []float64       `json:"f0_curve"`
	Units       []WorldlineUnit `json:"units"`
}

type WorldlineUnit struct {
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
	Modulation        float64                  `json:"modulation,omitempty"`
	Tempo             float64                  `json:"tempo,omitempty"`
	Envelope          []WorldlineEnvelopePoint `json:"envelope,omitempty"`
}

type WorldlineEnvelopePoint struct {
	XMS float64 `json:"x_ms"`
	Y   float64 `json:"y"`
}

// PitchCurve is the wire representation of a frame-level pitch contour.
type PitchCurve struct {
	FrameMS float64   `json:"frame_ms"`
	Cents   []float64 `json:"cents"`
}

// NeuralSynthesizerJobVersion is the version of the common neural score job.
const NeuralSynthesizerJobVersion = 1

// NeuralSynthesizerJob is the common v1 payload for providers that synthesize
// audio from a neural score. Options is provider-specific JSON by design: the
// score and resource names are common, while model controls belong to the
// selected provider implementation.
type NeuralSynthesizerJob struct {
	Version         int                `json:"version"`
	Contract        string             `json:"contract"`
	ContractVersion int                `json:"contract_version"`
	Score           engine.NeuralScore `json:"score"`
	Options         json.RawMessage    `json:"options,omitempty"`
	Resources       map[string]string  `json:"resources,omitempty"`
}
