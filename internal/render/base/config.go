package base

import "context"

type Config struct {
	TargetF0              *F0Track
	Context               context.Context
	WorldEnginePath       string
	ReleaseMS             float64
	ReleaseSet            bool
	LeadingPreutteranceMS float64
	IntonationStrength    float64
	ApplyPitch            bool
	World                 WorldOptions
	CVVCTiming            string
	CVVCTransitionGain    float64
	CVVCPreBoundaryFade   bool
	PitchCurve            *PitchCurve
	// 伸縮補正は日本語のモーラだけに適用する。
	StretchAdapt bool
	// StretchAdaptStrengthは伸縮補正の強度。0は既定1.0、負値は恒等。
	StretchAdaptStrength float64
}

// WorldOptionsはWORLDで描画するときの切り替え。
type WorldOptions struct {
	// 日本語のモーラ内を学習した時間配分に合わせる。
	TimingWarp bool
	// 日本語の子音前後に微細な音高変化を足す。言語に効くかは合成側で決めてから渡す。
	Microprosody bool
}

const (
	CVVCTimingSequential = "sequential"
)

const MaxIntonationStrength = 4.0

// DefaultReleaseMSは未指定時のリリース長。明示的な0にはReleaseSetを使う。
const DefaultReleaseMS = 20.0
