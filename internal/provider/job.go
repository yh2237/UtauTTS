package provider

import (
	"encoding/json"

	"utautts/internal/engine"
)

const (
	// CapabilityUnitRendererJobV1 marks the common unit-renderer job envelope.
	CapabilityUnitRendererJobV1 = "unit_renderer_job_v1"
	// CapabilityNeuralScoreJobV1 marks the common neural-synthesizer job
	// envelope. It lets older providers fail handshake and use legacy fallback.
	CapabilityNeuralScoreJobV1 = "neural_score_job_v1"
)

// UnitRendererJobVersion is the version of the host-owned unit-renderer job
// file. The transport request only carries paths to this file; the contract
// payload stays language-neutral and can be consumed by a non-Go provider.
const UnitRendererJobVersion = 1

// UnitRendererJob is the common v1 payload for providers that render a
// selected UTAU Unit Plan. ProviderPayload is reserved for a migration adapter
// that still needs implementation-specific input; normal external providers
// should consume Plan, Options, and Resources directly.
type UnitRendererJob struct {
	Version         int                 `json:"version"`
	Contract        string              `json:"contract"`
	ContractVersion int                 `json:"contract_version"`
	Plan            json.RawMessage     `json:"plan"`
	Options         UnitRendererOptions `json:"options"`
	Resources       map[string]string   `json:"resources,omitempty"`
	ProviderPayload json.RawMessage     `json:"provider_payload,omitempty"`
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
