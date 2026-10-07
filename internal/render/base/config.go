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
	// 伸縮補正は日本語のモーラだけに適用する。
	StretchAdapt bool
	// StretchAdaptStrengthは伸縮補正の強度。0は既定1.0、負値は恒等。
	StretchAdaptStrength float64
}

type ProviderOptions struct {
	Classic    ClassicOptions
	Worldline  WorldlineProviderOptions
	DiffSinger DiffSingerOptions
	// Goが知らないrenderer_settingsも、実装固有の値として保持する。
	Renderer            map[string]any
	RendererDiagnostics []string
}

// DiffSingerOptionsはDiffSinger推論の任意調整。0は既定値を使う。
type DiffSingerOptions struct {
	Steps       int64
	DurationMix float64
	PitchMix    float64
	Expr        float64
}

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

type WorldlineProviderOptions struct {
	// 原音区間ライブラリを使う。未指定は有効。
	SourcePhoneMapping *bool `json:"source_phone_mapping,omitempty"`
	// 試聴用の原音区間指定。通常の合成では未指定。
	ExperimentalSourceSpans map[int]SourceSpan `json:"-"`
	ExactLength             bool
	MixMode                 string
	GapRepairMode           string
	// 英語の語末破裂音を閉鎖と解放に分ける。nilは有効。
	E2A *bool
	// 日本語破裂音を保護する。nilは有効。
	E2B *bool
	// 日本語のモーラ内を学習した時間配分に合わせる。nilは有効。
	TimingWarp *bool `json:"timing_warp,omitempty"`
	// 日本語の子音前後に微細な音高変化を足す。nilは無効（引退）。
	Microprosody *bool `json:"microprosody,omitempty"`
}

func (options WorldlineProviderOptions) MicroprosodyEnabled() bool {
	return options.Microprosody != nil && *options.Microprosody
}

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
