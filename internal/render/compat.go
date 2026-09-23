package render

import (
	"context"

	"utautts/internal/audio"
	"utautts/internal/plan"
	"utautts/internal/provider"
	"utautts/internal/render/base"
)

// 共有層(base)の型を外部互換のため再公開する。
type (
	Config                   = base.Config
	ProviderOptions          = base.ProviderOptions
	ClassicOptions           = base.ClassicOptions
	DiffSingerOptions        = base.DiffSingerOptions
	WorldlineProviderOptions = base.WorldlineProviderOptions
	ResamplerExpression      = base.ResamplerExpression
	PitchCurve               = base.PitchCurve
	F0Track                  = base.F0Track
)

// 内部処理で使う共有型の別名。
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

// テストが参照する共有定数。
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

// ClearWAVCacheは音源更新後にキャッシュ済み録音を破棄する。
func ClearWAVCache() { base.ClearWAVCache() }

func estimateUnitPitch(unit plan.Unit, mono *audio.PCM) (float64, error) {
	return base.EstimateUnitPitch(unit, mono)
}

func effectiveUnitPitchFactor(unit plan.Unit, applyPitch bool) float64 {
	return base.EffectiveUnitPitchFactor(unit, applyPitch)
}

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

func medianFloat(values []float64) float64 { return base.MedianFloat(values) }

func identityFactors(size int) []float64 { return base.IdentityFactors(size) }

func nonzeroFloats(values []float64) []float64 { return base.NonzeroFloats(values) }

func analyzeIntonation(synthesisPlan *plan.Plan, timings []effectiveTiming, cache *sourceCache, strength float64) []float64 {
	return base.AnalyzeIntonation(synthesisPlan, timings, cache, strength)
}

func analyzeIntonationFromPitches(synthesisPlan *plan.Plan, timings []effectiveTiming, pitches []float64, strength float64) []float64 {
	return base.AnalyzeIntonationFromPitches(synthesisPlan, timings, pitches, strength)
}

func stabilizeSingleCVPitches(synthesisPlan *plan.Plan, values []float64) []float64 {
	return base.StabilizeSingleCVPitches(synthesisPlan, values)
}

func stabilizeWorldlinePitches(values []float64) []float64 {
	return base.StabilizeWorldlinePitches(values)
}

func measureWorldlinePitches(synthesisPlan *plan.Plan, cache *sourceCache) ([]float64, int, error) {
	return base.MeasureWorldlinePitches(synthesisPlan, cache)
}

func normalizeTiming(unit plan.Unit, releaseMS float64) effectiveTiming {
	return base.NormalizeTiming(unit, releaseMS)
}

func normalizePlanTiming(synthesisPlan *plan.Plan, unit plan.Unit, releaseMS float64) effectiveTiming {
	return base.NormalizePlanTiming(synthesisPlan, unit, releaseMS)
}

func normalizedPhoneTimingUnits(synthesisPlan *plan.Plan, releaseMS float64) []plan.Unit {
	return base.NormalizedPhoneTimingUnits(synthesisPlan, releaseMS)
}

func isVCVUnit(unit plan.Unit) bool { return base.IsVCVUnit(unit) }

func adaptStretchTiming(unit plan.Unit, timing effectiveTiming, releaseMS float64, enabled bool, strength float64) effectiveTiming {
	return base.AdaptStretchTiming(unit, timing, releaseMS, enabled, strength)
}

func onsetOverlapMS(onset string, preutterance, overlap float64) float64 {
	return base.OnsetOverlapMS(onset, preutterance, overlap)
}

func codaBoundaryOverlapMS(previousDuration, preutterance, overlap float64) (float64, float64, bool) {
	return base.CodaBoundaryOverlapMS(previousDuration, preutterance, overlap)
}

func singleCVOnset(synthesisPlan *plan.Plan, unit plan.Unit) string {
	return base.SingleCVOnset(synthesisPlan, unit)
}

func singleCVWorldOverlapMS(synthesisPlan *plan.Plan, unit plan.Unit, preutteranceMS float64) float64 {
	return base.SingleCVWorldOverlapMS(synthesisPlan, unit, preutteranceMS)
}

func singleCVBoundaryEligible(synthesisPlan *plan.Plan, previous, current renderedUnit) bool {
	return base.SingleCVBoundaryEligible(synthesisPlan, previous, current)
}

func singleCVMoraBoundaryEligible(synthesisPlan *plan.Plan, position int) bool {
	return base.SingleCVMoraBoundaryEligible(synthesisPlan, position)
}

func singleCVProtectedOnset(synthesisPlan *plan.Plan, unit plan.Unit) bool {
	return base.SingleCVProtectedOnset(synthesisPlan, unit)
}

func limitLeadingPreutterance(required, maximum float64) float64 {
	return base.LimitLeadingPreutterance(required, maximum)
}

func fadeInDurationMS(timing effectiveTiming) float64 { return base.FadeInDurationMS(timing) }

func openUtauPhoneTimings(units []plan.Unit, cvvcTiming string) ([]openUtauPhoneTiming, float64) {
	return base.OpenUtauPhoneTimings(units, cvvcTiming)
}

func openUtauPhoneTimingsWithCoda(units []plan.Unit, cvvcTiming string, protectCoda bool) ([]openUtauPhoneTiming, float64) {
	return base.OpenUtauPhoneTimingsWithCoda(units, cvvcTiming, protectCoda)
}

func openUtauEnvelopeFromTiming(unit plan.Unit, timing openUtauPhoneTiming) []worldlineEnvelopePoint {
	return base.OpenUtauEnvelopeFromTiming(unit, timing)
}

func cvvcPreBoundaryEnvelope(points []worldlineEnvelopePoint, timing openUtauPhoneTiming) []worldlineEnvelopePoint {
	return base.CVVCPreBoundaryEnvelope(points, timing)
}

func providerPitchCurve(curve *PitchCurve) *provider.PitchCurve {
	return base.ProviderPitchCurve(curve)
}

func speechRetime(source []float64, targetFrames, sourceOnset, sourceFixed, targetOnset, targetFixed, rate int, stop bool) ([]float64, int, bool) {
	return base.SpeechRetime(source, targetFrames, sourceOnset, sourceFixed, targetOnset, targetFixed, rate, stop)
}

func speechStop(p *plan.Plan, unit plan.Unit) bool { return base.SpeechStop(p, unit) }

func speechPitchAnchor(sourceFrames, anchor int, basePitch float64, curve *PitchCurve, startMS, spanMS float64) int {
	return base.SpeechPitchAnchor(sourceFrames, anchor, basePitch, curve, startMS, spanMS)
}

func speechVowelJoin(p *plan.Plan, previous, current renderedUnit) bool {
	return base.SpeechVowelJoin(p, previous, current)
}

func codaReleaseStop(u plan.Unit) bool { return base.CodaReleaseStop(u) }

func retimeWithCompressedPrefixUsing(source []float64, targetFrames, sourcePrefixFrames, targetPrefixFrames, sampleRate int, stretch func([]float64, int, int) ([]float64, error)) ([]float64, error) {
	return base.RetimeWithCompressedPrefixUsing(source, targetFrames, sourcePrefixFrames, targetPrefixFrames, sampleRate, stretch)
}

func stretchPreservingPrefixUsing(source []float64, targetFrames, prefixFrames, sampleRate int, stretch func([]float64, int, int) ([]float64, error)) ([]float64, error) {
	return base.StretchPreservingPrefixUsing(source, targetFrames, prefixFrames, sampleRate, stretch)
}

func wsolaStretch(source []float64, targetFrames, sampleRate int) ([]float64, error) {
	return base.WSOLAStretch(source, targetFrames, sampleRate)
}

// StretchWSOLAは実験・診断処理から標準WSOLAを再利用する。
func StretchWSOLA(source []float64, targetFrames, sampleRate int) []float64 {
	return base.StretchWSOLA(source, targetFrames, sampleRate)
}

// StretchWSOLAAnchoredは連続波形を分割せず、指定した時間写像に沿って伸縮する。
func StretchWSOLAAnchored(source []float64, targetFrames, sampleRate int, sourceAnchors, targetAnchors []int) []float64 {
	return base.StretchWSOLAAnchored(source, targetFrames, sampleRate, sourceAnchors, targetAnchors)
}

func msToFrames(ms float64, sampleRate int) int { return base.MsToFrames(ms, sampleRate) }

func msToFramesSigned(ms float64, sampleRate int) int { return base.MsToFramesSigned(ms, sampleRate) }

func framesToMS(frames, sampleRate int) float64 { return base.FramesToMS(frames, sampleRate) }

func pcmFloats(data []int16) []float64 { return base.PcmFloats(data) }

func floatPCM(data []float64) []int16 { return base.FloatPCM(data) }

func smoothstep(value float64) float64 { return base.Smoothstep(value) }

func contextError(ctx context.Context) error { return base.ContextError(ctx) }

func toMono(pcm *audio.PCM) *audio.PCM { return base.ToMono(pcm) }

func resampleRate(pcm *audio.PCM, targetRate int) *audio.PCM {
	return base.ResampleRate(pcm, targetRate)
}
