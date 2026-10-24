package base

import (
	"context"

	"github.com/yh2237/UtauTTS/internal/settings"
)

type Config struct {
	TargetF0              *F0Track
	Context               context.Context
	WorldEnginePath       string
	ReleaseMS             float64
	ReleaseSet            bool
	LeadingPreutteranceMS float64
	IntonationStrength    float64
	ApplyPitch            bool
	ProviderOptions       ProviderOptions
	CVVCTiming            string
	CVVCTransitionGain    float64
	CVVCPreBoundaryFade   bool
	PitchCurve            *PitchCurve
	// 伸縮補正は日本語のモーラだけに適用する。
	StretchAdapt bool
	// StretchAdaptStrengthは伸縮補正の強度。0は既定1.0、負値は恒等。
	StretchAdaptStrength float64
}

type ProviderOptions struct {
	Worldline WorldlineProviderOptions
	// Goが知らないrenderer_settingsも、実装固有の値として保持する。
	Renderer            map[string]any
	RendererDiagnostics []string
}

type WorldlineProviderOptions struct {
	// 原音区間ライブラリを使う。未指定は有効。
	SourcePhoneMapping *bool `json:"source_phone_mapping,omitempty"`
	// 試聴用の原音区間指定。通常の合成では未指定。
	ExactLength bool
	// 日本語のモーラ内を学習した時間配分に合わせる。nilは有効。
	TimingWarp *bool `json:"timing_warp,omitempty"`
	// 日本語の子音前後に微細な音高変化を足す。nilは有効。
	Microprosody *bool `json:"microprosody,omitempty"`
}

func (options WorldlineProviderOptions) MicroprosodyEnabled() bool {
	if options.Microprosody == nil {
		return settings.Bool("microprosody")
	}
	return *options.Microprosody
}

func (options WorldlineProviderOptions) TimingWarpEnabled() bool {
	if options.TimingWarp == nil {
		return settings.Bool("timing_warp")
	}
	return *options.TimingWarp
}

const (
	CVVCTimingSequential = "sequential"
)

const MaxIntonationStrength = 4.0

// DefaultReleaseMSは未指定時のリリース長。明示的な0にはReleaseSetを使う。
const DefaultReleaseMS = 20.0
