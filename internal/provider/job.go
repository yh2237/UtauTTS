package provider

import "encoding/json"

const (
	CapabilityUnitRendererJobV2 = "unit_renderer_job_v2"
	CapabilityNeuralScoreJobV1  = "neural_score_job_v1"
)

const UnitRendererJobVersion = 2

type UnitRendererJob struct {
	Version         int                 `json:"version"`
	Contract        string              `json:"contract"`
	ContractVersion int                 `json:"contract_version"`
	Plan            json.RawMessage     `json:"plan"`
	Options         UnitRendererOptions `json:"options"`
	Resources       map[string]string   `json:"resources,omitempty"`
}

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
const CapabilitySpeechAnchorsV1 = "speech_anchors_v1"

// SpeechAnchorは切り出し原音の時刻を出力内の時刻へ対応付ける。
type SpeechAnchor struct {
	SourceMS float64 `json:"source_ms"`
	TargetMS float64 `json:"target_ms"`
}

// WorldSpeechTimingは切り出し後の音源を基準とする位置を示す。
type WorldSpeechTiming struct {
	Anchors                   []SpeechAnchor `json:"anchors,omitempty"`
	CodaRelease               bool           `json:"coda_release,omitempty"`
	SeparateRelease           bool           `json:"separate_release,omitempty"`
	PreserveStopOnly          bool           `json:"preserve_stop_only,omitempty"`
	UnitIndex                 int            `json:"unit_index"`
	SourceOnsetMS             float64        `json:"source_onset_ms"`
	SourceTransientMS         float64        `json:"source_transient_ms,omitempty"`
	SourceTransientDurationMS float64        `json:"source_transient_duration_ms,omitempty"`
	TargetOnsetMS             float64        `json:"target_onset_ms"`
	ReleaseMS                 float64        `json:"release_ms,omitempty"`
	ProtectStop               bool           `json:"protect_stop,omitempty"`
	VowelJoin                 bool           `json:"vowel_join,omitempty"`
	TargetFixedMS             float64        `json:"target_fixed_ms,omitempty"`
	TargetJoinMS              float64        `json:"target_join_ms,omitempty"`
	TransitionLeftPhone       string         `json:"transition_left_phone,omitempty"`
	TransitionRightPhone      string         `json:"transition_right_phone,omitempty"`
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

type PitchCurve struct {
	FrameMS float64   `json:"frame_ms"`
	Cents   []float64 `json:"cents"`
}

const NeuralSynthesizerJobVersion = 1

type NeuralSynthesizerJob struct {
	Version         int               `json:"version"`
	Contract        string            `json:"contract"`
	ContractVersion int               `json:"contract_version"`
	Score           NeuralScore       `json:"score"`
	Options         json.RawMessage   `json:"options,omitempty"`
	Resources       map[string]string `json:"resources,omitempty"`
}

// NeuralScoreはneural-synthesizer Providerへ渡す音素・長さ・F0の楽譜。
type NeuralScore struct {
	Symbols   []string  `json:"symbols"`
	Durations []int64   `json:"durations"`
	F0        []float32 `json:"f0"`
	MIDI      int       `json:"midi"`
	// NoteMIDIは音符(単語)ごとのMIDI、PhMIDIは音素ごとのMIDI。話声向けにF0から求める。
	NoteMIDI []float32 `json:"note_midi,omitempty"`
	PhMIDI   []int64   `json:"ph_midi,omitempty"`
	// StepsとDurationPredictorMixはDiffSinger推論の任意調整値。0なら既定値を使う。
	Steps                int64   `json:"steps,omitempty"`
	DurationPredictorMix float32 `json:"duration_predictor_mix,omitempty"`
	// ExprはDiffSingerの表現力（既定1.0）。0なら既定値を使う。
	Expr              float32 `json:"expr,omitempty"`
	WordDiv           []int64 `json:"word_div,omitempty"`
	WordDur           []int64 `json:"word_dur,omitempty"`
	NoteRest          []bool  `json:"note_rest,omitempty"`
	UsePitchPredictor bool    `json:"use_pitch_predictor,omitempty"`
	PitchPredictorMix float32 `json:"pitch_predictor_mix,omitempty"`
}
