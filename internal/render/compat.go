package render

import (
	"context"

	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/provider"
	"github.com/yh2237/UtauTTS/internal/render/base"
)

type (
	Config                   = base.Config
	ProviderOptions          = base.ProviderOptions
	DiffSingerOptions        = base.DiffSingerOptions
	WorldlineProviderOptions = base.WorldlineProviderOptions
	PitchCurve               = base.PitchCurve
	F0Track                  = base.F0Track
)

type (
	effectiveTiming        = base.EffectiveTiming
	renderedUnit           = base.RenderedUnit
	worldlineEnvelopePoint = base.WorldlineEnvelopePoint
	openUtauPhoneTiming    = base.OpenUtauPhoneTiming
	sourceCache            = base.SourceCache
)

const (
	CVVCTimingSequential  = base.CVVCTimingSequential
	MaxIntonationStrength = base.MaxIntonationStrength
	DefaultReleaseMS      = base.DefaultReleaseMS
)

const (
	singleCVMinimumVowelTailMS    = base.SingleCVMinimumVowelTailMS
	singleCVVowelTailRatio        = base.SingleCVVowelTailRatio
	singleCVSameVowelOverlapMS    = base.SingleCVSameVowelOverlapMS
	singleCVDefaultVowelOverlapMS = base.SingleCVDefaultVowelOverlapMS
	codaBoundaryMinTailMS         = base.CodaBoundaryMinTailMS
	codaBoundaryMaxOverlapMS      = base.CodaBoundaryMaxOverlapMS
	stretchAdaptMaxRatio          = base.StretchAdaptMaxRatio
	stretchAdaptMinTailMS         = base.StretchAdaptMinTailMS
	stretchAdaptMinTailRatio      = base.StretchAdaptMinTailRatio
	stretchAdaptStrengthLimit     = base.StretchAdaptStrengthLimit
)

func newSourceCache() sourceCache { return base.NewSourceCache() }

func ClearWAVCache() { base.ClearWAVCache() }

func pitchCurveFactorAt(curve *PitchCurve, timeMS float64) float64 {
	return base.PitchCurveFactorAt(curve, timeMS)
}

func clampPitchFactor(factor float64) float64 { return base.ClampPitchFactor(factor) }

func resampleForPitch(source []float64, factor float64) []float64 {
	return base.ResampleForPitch(source, factor)
}

func resampleForPitchCurve(source []float64, baseFactor float64, curve *PitchCurve, startMS, spanMS float64) []float64 {
	return base.ResampleForPitchCurve(source, baseFactor, curve, startMS, spanMS)
}

func identityFactors(size int) []float64 { return base.IdentityFactors(size) }

func analyzeIntonation(synthesisPlan *plan.Plan, timings []effectiveTiming, cache *sourceCache, strength float64) []float64 {
	return base.AnalyzeIntonation(synthesisPlan, timings, cache, strength)
}

func normalizePlanTiming(synthesisPlan *plan.Plan, unit plan.Unit, releaseMS float64) effectiveTiming {
	return base.NormalizePlanTiming(synthesisPlan, unit, releaseMS)
}

func adaptStretchTiming(unit plan.Unit, timing effectiveTiming, releaseMS float64, enabled bool, strength float64) effectiveTiming {
	return base.AdaptStretchTiming(unit, timing, releaseMS, enabled, strength)
}

func singleCVOnset(synthesisPlan *plan.Plan, unit plan.Unit) string {
	return base.SingleCVOnset(synthesisPlan, unit)
}

func singleCVBoundaryEligible(synthesisPlan *plan.Plan, previous, current renderedUnit) bool {
	return base.SingleCVBoundaryEligible(synthesisPlan, previous, current)
}

func singleCVProtectedOnset(synthesisPlan *plan.Plan, unit plan.Unit) bool {
	return base.SingleCVProtectedOnset(synthesisPlan, unit)
}

func limitLeadingPreutterance(required, maximum float64) float64 {
	return base.LimitLeadingPreutterance(required, maximum)
}

func fadeInDurationMS(timing effectiveTiming) float64 { return base.FadeInDurationMS(timing) }

func providerPitchCurve(curve *PitchCurve) *provider.PitchCurve {
	return base.ProviderPitchCurve(curve)
}

func speechVowelJoin(p *plan.Plan, previous, current renderedUnit) bool {
	return base.SpeechVowelJoin(p, previous, current)
}

func retimeWithCompressedPrefixUsing(source []float64, targetFrames, sourcePrefixFrames, targetPrefixFrames, sampleRate int, stretch func([]float64, int, int) ([]float64, error)) ([]float64, error) {
	return base.RetimeWithCompressedPrefixUsing(source, targetFrames, sourcePrefixFrames, targetPrefixFrames, sampleRate, stretch)
}

func wsolaStretch(source []float64, targetFrames, sampleRate int) ([]float64, error) {
	return base.WSOLAStretch(source, targetFrames, sampleRate)
}

func msToFrames(ms float64, sampleRate int) int { return base.MsToFrames(ms, sampleRate) }

func msToFramesSigned(ms float64, sampleRate int) int { return base.MsToFramesSigned(ms, sampleRate) }

func framesToMS(frames, sampleRate int) float64 { return base.FramesToMS(frames, sampleRate) }

func pcmFloats(data []int16) []float64 { return base.PcmFloats(data) }

func floatPCM(data []float64) []int16 { return base.FloatPCM(data) }

func smoothstep(value float64) float64 { return base.Smoothstep(value) }

func contextError(ctx context.Context) error { return base.ContextError(ctx) }
