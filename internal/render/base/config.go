package base

import (
	"context"

	"utautts/internal/engine"
)

// Configはrenderer実行に必要な設定を保持する。共有層の型。
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

// ProviderOptionsは特定provider固有の設定を保持する。無関係なproviderの実行パスやスイッチが混ざるのを防ぐ。
type ProviderOptions struct {
	Classic    ClassicOptions
	Worldline  WorldlineProviderOptions
	DiffSinger DiffSingerOptions
	// Rendererはmanifestのrenderer_settingsのうちGoが既知でないprovider固有値を保持する。
	// providerが使わなくても無害で、診断としてそのまま参照できる。
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
	SpeechPitchReference bool
	ExactLength          bool
	MixMode              string
	GapRepairMode        string
	// E2Aは英語停止codaの閉鎖/解放分離(E2a)を有効にする。nilは既定ON。
	E2A *bool
	// E2Bは日本語破裂音の過渡音ゲート一般化(E2b)を有効にする。nilは既定ON。
	E2B *bool
}

// E2AEnabledはE2aの実効値を返す。未指定は既定ON。
func (options WorldlineProviderOptions) E2AEnabled() bool {
	return options.E2A == nil || *options.E2A
}

// E2BEnabledはE2bの実効値を返す。未指定は既定ON。
func (options WorldlineProviderOptions) E2BEnabled() bool {
	return options.E2B == nil || *options.E2B
}

// Resourceは解決済みengineからresourceを引く。
func (cfg Config) Resource(key engine.ResourceKey) string {
	return cfg.Engine.Resource(key)
}

// ProviderIDは実行providerを返す。
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

// MaxIntonationStrengthはユーザー向けイントネーション制御の上限値。
const MaxIntonationStrength = 4.0

// DefaultReleaseMSは未指定時のリリース長。明示的な0にはReleaseSetを使う。
const DefaultReleaseMS = 20.0
