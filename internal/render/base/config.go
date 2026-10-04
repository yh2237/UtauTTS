package base

import (
	"context"

	"utautts/internal/engine"
)

type Config struct {
	TargetF0                *F0Track
	Context                 context.Context
	Engine                  engine.ResolvedEngine
	ReleaseMS               float64
	ReleaseSet              bool
	LeadingPreutteranceMS   float64
	IntonationStrength      float64
	ApplyPitch              bool
	Backend                 string
	ProviderOptions         ProviderOptions
	BoundaryBridgeMS        float64
	BoundaryBridgeThreshold float64
	CVVCTiming              string
	CVVCTransitionGain      float64
	CVVCPreBoundaryFade     bool
	PitchCurve              *PitchCurve
	// StretchAdaptは伸縮の音源適応(C3a)を有効にする。日本語のモーラだけを対象にする。
	StretchAdapt bool
	// StretchAdaptStrengthは伸縮補正の強度。0は既定1.0、負値は恒等。
	StretchAdaptStrength float64
}

type ProviderOptions struct {
	Classic    ClassicOptions
	Worldline  WorldlineProviderOptions
	DiffSinger DiffSingerOptions
	// Goが知らないrenderer_settingsも、実装固有の値として保持する。
	Renderer map[string]any
	// RendererDiagnosticsはrenderer_settingsの型不一致などの非致命的な問題を記録する。
	RendererDiagnostics []string
}

// DiffSingerOptionsはDiffSinger推論の任意調整。0は既定値を使う。
type DiffSingerOptions struct {
	Steps       int64
	DurationMix float64
	PitchMix    float64
	Expr        float64
}

// ClassicOptionsは外部UTAUのresampler/wavtool設定。
type ClassicOptions struct {
	ResamplerPath        string
	WavtoolPath          string
	Velocity             int
	VelocitySet          bool
	Flags                string
	Modulation           int
	ModulationSet        bool
	Tempo                float64
	ResamplerExpressions []ResamplerExpression
}

// WorldlineProviderOptionsはWORLD専用のホスト制御。残りのprovider入力はWORLD jobが持つ。
type WorldlineProviderOptions struct {
	// 原音区間ライブラリを使う。未指定は有効。
	SourcePhoneMapping *bool `json:"source_phone_mapping,omitempty"`
	// 試聴用の原音区間指定。通常の合成では未指定。
	ExperimentalSourceSpans map[int]SourceSpan `json:"-"`
	ExactLength             bool
	MixMode                 string
	GapRepairMode           string
	// E2Aは英語停止codaの閉鎖/解放分離(E2a)を有効にする。nilは既定ON。
	E2A *bool
	// E2Bは日本語破裂音の過渡音ゲート一般化(E2b)を有効にする。nilは既定ON。
	E2B *bool
	// TimingWarpは日本語の出力を、学習した読み上げの動きに合わせてモーラの中だけ時間伸縮する。nilは既定ON。
	TimingWarp *bool `json:"timing_warp,omitempty"`
	// Microprosodyは日本語のF0曲線へ、子音の直後の小さな上下（自然な読み上げで測った値）を足す。nilは既定ON。
	Microprosody *bool `json:"microprosody,omitempty"`
}

// MicroprosodyEnabledは子音の前後の微細韻律が有効か。
func (options WorldlineProviderOptions) MicroprosodyEnabled() bool {
	return options.Microprosody == nil || *options.Microprosody
}

// TimingWarpEnabledは時間伸縮が有効か。
func (options WorldlineProviderOptions) TimingWarpEnabled() bool {
	return options.TimingWarp == nil || *options.TimingWarp
}

func (options WorldlineProviderOptions) SeparateCodaReleaseEnabled() bool {
	return options.E2A == nil || *options.E2A
}

func (options WorldlineProviderOptions) JapaneseStopProtectionEnabled() bool {
	return options.E2B == nil || *options.E2B
}

// E2AEnabledは既存ツールとの互換用。
func (options WorldlineProviderOptions) E2AEnabled() bool {
	return options.SeparateCodaReleaseEnabled()
}

// E2BEnabledは既存ツールとの互換用。
func (options WorldlineProviderOptions) E2BEnabled() bool {
	return options.JapaneseStopProtectionEnabled()
}

func (cfg Config) Resource(key engine.ResourceKey) string {
	return cfg.Engine.Resource(key)
}

func (cfg Config) ProviderID() engine.ProviderID {
	if cfg.Engine.Provider.ID != "" {
		return cfg.Engine.Provider.ID
	}
	return engine.ProviderID(cfg.Backend)
}

// ResamplerExpressionは位置ごとのresampler上書き。
type ResamplerExpression struct {
	Position   int      `json:"position"`
	Velocity   *int     `json:"velocity,omitempty"`
	Volume     *int     `json:"volume,omitempty"`
	Flags      *string  `json:"flags,omitempty"`
	Modulation *int     `json:"modulation,omitempty"`
	Tempo      *float64 `json:"tempo,omitempty"`
}

const (
	CVVCTimingSequential = "sequential"
)

const MaxIntonationStrength = 4.0

// DefaultReleaseMSは未指定時のリリース長。明示的な0にはReleaseSetを使う。
const DefaultReleaseMS = 20.0
