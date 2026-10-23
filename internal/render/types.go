package render

import "github.com/yh2237/UtauTTS/internal/render/base"

type (
	Config                   = base.Config
	ProviderOptions          = base.ProviderOptions
	WorldlineProviderOptions = base.WorldlineProviderOptions
	PitchCurve               = base.PitchCurve
	F0Track                  = base.F0Track
)

const (
	CVVCTimingSequential  = base.CVVCTimingSequential
	MaxIntonationStrength = base.MaxIntonationStrength
	DefaultReleaseMS      = base.DefaultReleaseMS
)

func ClearWAVCache() { base.ClearWAVCache() }
