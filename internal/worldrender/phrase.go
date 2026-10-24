package worldrender

// FramePeriodMSはF0曲線とWORLD解析のフレーム間隔。
const FramePeriodMS = 10.0

// SourceEndMSは切り出し範囲の終端時刻を返す。cutoffが負ならoffsetからの長さ指定。
func SourceEndMS(sourceDurationMS, offsetMS, cutoffMS float64) float64 {
	if cutoffMS < 0 {
		return offsetMS - cutoffMS
	}
	return sourceDurationMS - cutoffMS
}

// Phraseは1つの句をWORLDで描画するための入力。
type Phrase struct {
	SampleRate int
	F0Curve    []float64
	Units      []Unit
	// TimingWarpは日本語の時間伸縮の入力。nilは無効。
	TimingWarp *TimingWarp
}

// SpeechAnchorは切り出し原音の時刻を出力内の時刻へ対応付ける。
type SpeechAnchor struct {
	SourceMS float64 `json:"source_ms"`
	TargetMS float64 `json:"target_ms"`
}

// SpeechTimingは切り出し後の音源を基準とする位置を示す。
type SpeechTiming struct {
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

type SpeechResult struct {
	UnitIndex         int     `json:"unit_index"`
	RetimeApplied     bool    `json:"retime_applied"`
	TargetFixedMS     float64 `json:"target_fixed_ms"`
	JoinApplied       bool    `json:"join_applied"`
	TransitionApplied bool    `json:"transition_applied,omitempty"`
	StopBurstApplied  bool    `json:"stop_burst_applied,omitempty"`
	StopBurstGain     float64 `json:"stop_burst_gain,omitempty"`
}

// TimingWarpは時間伸縮の入力。タイムラインは本体で計算し、bridgeは消費するだけ。
type TimingWarp struct {
	Strength float64          `json:"strength"`
	Language string           `json:"language,omitempty"`
	Spans    []TimingWarpSpan `json:"spans"`
	Starts   []float64        `json:"starts,omitempty"`
	Ends     []float64        `json:"ends,omitempty"`
}

// TimingWarpSpanはフレーズ先頭基準の音素区間（秒）。
type TimingWarpSpan struct {
	Label string  `json:"label"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}
