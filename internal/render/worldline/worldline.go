package worldline

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yh2237/UtauTTS/internal/audio"
	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/render/base"
	"github.com/yh2237/UtauTTS/internal/speechtiming"
	"github.com/yh2237/UtauTTS/internal/worldrender"
)

// nilなら計測ログを出さない。
var Trace func(message string)

func traceMark(start *time.Time, label string) {
	if Trace == nil {
		return
	}
	now := time.Now()
	Trace(fmt.Sprintf("%s: %.1fms", label, float64(now.Sub(*start).Microseconds())/1000))
	*start = now
}

const worldlineFrameMS = worldrender.FramePeriodMS

// RenderはPlanをWORLDで描画する。
func Render(synthesisPlan *plan.Plan, cfg base.Config) (*audio.PCM, error) {
	return renderWorldlineEngine(synthesisPlan, cfg)
}

type worldlineManifestUnit struct {
	Speech            *worldrender.SpeechTiming     `json:"speech,omitempty"`
	LegacyMix         bool                          `json:"legacy_mix,omitempty"`
	GapRepair         bool                          `json:"gap_repair,omitempty"`
	CacheKey          string                        `json:"cache_key,omitempty"`
	Source            string                        `json:"source"`
	FRQPath           string                        `json:"frq_path,omitempty"`
	PositionMS        float64                       `json:"position_ms"`
	SkipMS            float64                       `json:"skip_ms"`
	LengthMS          float64                       `json:"length_ms"`
	FadeInMS          float64                       `json:"fade_in_ms"`
	FadeOutMS         float64                       `json:"fade_out_ms"`
	OffsetMS          float64                       `json:"offset_ms"`
	RequiredLengthMS  float64                       `json:"required_length_ms"`
	ConsonantMS       float64                       `json:"consonant_ms"`
	CutoffMS          float64                       `json:"cutoff_ms"`
	Tone              int                           `json:"tone"`
	ConsonantVelocity float64                       `json:"consonant_velocity"`
	PitchStartMS      float64                       `json:"pitch_start_ms,omitempty"`
	PitchLengthMS     float64                       `json:"pitch_length_ms,omitempty"`
	Volume            float64                       `json:"volume,omitempty"`
	VolumeSet         bool                          `json:"volume_set,omitempty"`
	Modulation        float64                       `json:"modulation,omitempty"`
	Tempo             float64                       `json:"tempo,omitempty"`
	EnergyFactor      float64                       `json:"energy_factor,omitempty"`
	Envelope          []base.WorldlineEnvelopePoint `json:"envelope,omitempty"`
}

func renderWorldlineEngine(synthesisPlan *plan.Plan, cfg base.Config) (*audio.PCM, error) {
	if synthesisPlan == nil || len(synthesisPlan.Units) == 0 {
		return nil, errors.New("empty synthesis plan")
	}
	started := time.Now()
	if err := normalizeCVVCConfig(synthesisPlan, &cfg); err != nil {
		return nil, err
	}
	worldEnginePath, err := resolveWorldEngine(cfg.WorldEnginePath)
	if err != nil {
		return nil, err
	}
	legacyMix := legacyJapaneseContinuousMix(synthesisPlan)
	cache := base.NewSourceCache()
	timing := prepareWorldlineTiming(synthesisPlan, cfg)
	traceMark(&started, "timing")
	pitch, err := prepareWorldlinePitch(synthesisPlan, cfg, &cache, timing)
	if err != nil {
		return nil, err
	}
	traceMark(&started, "pitches")
	// 一時ファイルは合成ごとに隔離する。
	tempDir, err := os.MkdirTemp("", "utautts-worldline-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)
	normalizedSources, err := normalizeWorldlineSources(synthesisPlan, &cache, pitch.sampleRate, tempDir)
	if err != nil {
		return nil, err
	}
	phrase := worldrender.Phrase{
		SampleRate: pitch.sampleRate,
		F0Curve:    pitch.f0Curve,
		TimingWarp: timingWarpJob(synthesisPlan, cfg, len(pitch.f0Curve)),
	}
	units := worldlineUnitBuilder{
		plan: synthesisPlan, cfg: cfg, cache: &cache, timing: timing, pitch: pitch,
		normalizedSources: normalizedSources, legacyMix: legacyMix,
	}
	for i := range synthesisPlan.Units {
		if synthesisPlan.Units[i].Silent {
			continue
		}
		item, err := units.build(i)
		if err != nil {
			return nil, err
		}
		phrase.Units = append(phrase.Units, item)
	}
	traceMark(&started, "units")
	pcm, err := renderWorldline(synthesisPlan, cfg, phrase, worldEnginePath, &started)
	if err != nil {
		return nil, err
	}
	minimumFrames := base.MsToFrames(synthesisPlan.DurationMS+cfg.ReleaseMS+timing.leadingMS, pcm.SampleRate)
	if len(pcm.Data) < minimumFrames {
		pcm.Data = append(pcm.Data, make([]int16, minimumFrames-len(pcm.Data))...)
	}
	return pcm, nil
}

func normalizeCVVCConfig(synthesisPlan *plan.Plan, cfg *base.Config) error {
	if cfg.CVVCTiming == "" {
		cfg.CVVCTiming = base.CVVCTimingSequential
	}
	if cfg.CVVCTiming != base.CVVCTimingSequential {
		return fmt.Errorf("unknown CVVC timing mode %q", cfg.CVVCTiming)
	}
	if cfg.CVVCTransitionGain == 0 {
		cfg.CVVCTransitionGain = 1
	}
	if cfg.CVVCTransitionGain < 0 || cfg.CVVCTransitionGain > 1 {
		return fmt.Errorf("CVVC transition gain must be between 0 and 1; got %.3f", cfg.CVVCTransitionGain)
	}
	synthesisPlan.CVVCTiming = cfg.CVVCTiming
	synthesisPlan.CVVCTransitionGain = cfg.CVVCTransitionGain
	synthesisPlan.CVVCPreBoundaryFade = cfg.CVVCPreBoundaryFade
	return nil
}

type worldlineTimingResult struct {
	units     []base.EffectiveTiming
	phones    []base.OpenUtauPhoneTiming
	leadingMS float64
}

func prepareWorldlineTiming(synthesisPlan *plan.Plan, cfg base.Config) worldlineTimingResult {
	phoneUnits := base.NormalizedPhoneTimingUnits(synthesisPlan, cfg.ReleaseMS)
	if synthesisPlan.SingleCV {
		for index := range phoneUnits {
			if phoneUnits[index].Silent || phoneUnits[index].Role != "mora" {
				continue
			}
			phoneUnits[index].OverlapMS = base.SingleCVWorldOverlapMS(synthesisPlan, phoneUnits[index], phoneUnits[index].PreutteranceMS)
			if singleCVLegato(synthesisPlan, phoneUnits[index]) {
				legato := math.Min(singleCVLegatoMS, phoneUnits[index].PreutteranceMS)
				phoneUnits[index].PreutteranceMS = legato
				phoneUnits[index].OverlapMS = legato
			}
		}
	}
	phoneTimings, phraseStartMS := base.OpenUtauPhoneTimingsWithCoda(phoneUnits, cfg.CVVCTiming, true)
	result := worldlineTimingResult{
		units:     make([]base.EffectiveTiming, len(synthesisPlan.Units)),
		phones:    phoneTimings,
		leadingMS: base.LimitLeadingPreutterance(math.Max(0, -phraseStartMS), cfg.LeadingPreutteranceMS),
	}
	synthesisPlan.LeadingMarginMS = result.leadingMS
	for i := range synthesisPlan.Units {
		unit := &synthesisPlan.Units[i]
		unit.SpeechRetimeApplied = false
		unit.SpeechJoinApplied = false
		unit.SpeechTransitionApplied = false
		unit.StopBurstApplied = false
		unit.StopBurstGain = 0
		unit.StopBurstReason = "not-required"
		unit.WorldRenderMode = plan.WorldRenderModeAdaptive
		unit.WorldRenderReason = "adaptive-default"
		unit.WorldGapRepairEligible = false
		unit.WorldGapRepairReason = "not-required"
		timing := worldlineTiming(synthesisPlan, *unit, cfg.ReleaseMS)
		timing = base.AdaptStretchTiming(*unit, timing, cfg.ReleaseMS, cfg.StretchAdapt, cfg.StretchAdaptStrength)
		if len(phoneTimings) == len(synthesisPlan.Units) && !unit.Silent {
			timing.PreutteranceMS = phoneTimings[i].Preutter
			timing.OverlapMS = phoneTimings[i].Overlap
			unit.CodaBoundaryLimited = phoneTimings[i].CodaLimited
			// 伸縮補正で動かした固定部は、otoの値に戻さない。
			if (unit.Role != "mora" || !synthesisPlan.SingleCV) && !timing.StretchAdapted {
				timing.ConsonantMS = unit.ConsonantMS
				timing.Scale = 1
			}
		}
		result.units[i] = timing
		unit.TimingScale = timing.Scale
		unit.EffectivePreutteranceMS = timing.PreutteranceMS
		unit.EffectiveConsonantMS = timing.ConsonantMS
		unit.EffectiveOverlapMS = timing.OverlapMS
		unit.CVTimingApplied = timing.CVApplied
		unit.CVTimingWarnings = append([]string(nil), timing.CVWarnings...)
		unit.StretchAdapted = timing.StretchAdapted
		unit.StretchLimitReason = timing.StretchLimitReason
		unit.IntonationFactor = 1
	}
	return result
}

type worldlinePitchResult struct {
	sampleRate   int
	pitches      []float64
	intonation   []float64
	pitchFactors []float64
	reference    float64
	f0Curve      []float64
}

func prepareWorldlinePitch(synthesisPlan *plan.Plan, cfg base.Config, cache *base.SourceCache, timing worldlineTimingResult) (worldlinePitchResult, error) {
	pitches, sampleRate, err := base.MeasureWorldlinePitches(synthesisPlan, cache)
	if err != nil {
		return worldlinePitchResult{}, err
	}
	intonation := base.IdentityFactors(len(synthesisPlan.Units))
	if cfg.ApplyPitch {
		intonation = base.AnalyzeIntonationFromPitches(synthesisPlan, timing.units, pitches, cfg.IntonationStrength)
	}
	reference := base.MedianFloat(base.NonzeroFloats(pitches))
	if reference <= 0 {
		reference = 220
	}
	pitchFactors := make([]float64, len(synthesisPlan.Units))
	for i, unit := range synthesisPlan.Units {
		pitchFactors[i] = intonation[i] * base.EffectiveUnitPitchFactor(unit, cfg.ApplyPitch)
	}
	base.ApplyTonePitch(synthesisPlan, pitchFactors, reference)
	frameMS := worldlineFrameMS
	curveStartMS := -timing.leadingMS
	curveDurationMS := synthesisPlan.DurationMS + cfg.ReleaseMS + timing.leadingMS
	f0Curve := worldlineF0CurveAtOffset(synthesisPlan, pitches, pitchFactors, reference,
		max(2, int(math.Ceil(curveDurationMS/frameMS))+2), frameMS, curveStartMS)
	for frame := range f0Curve {
		f0Curve[frame] *= base.PitchCurveFactorAt(cfg.PitchCurve, curveStartMS+float64(frame)*frameMS)
	}
	if cfg.ApplyPitch && cfg.ProviderOptions.Worldline.MicroprosodyEnabled() && microprosodyAppliesTo(synthesisPlan.Language) {
		applyMicroprosody(synthesisPlan, f0Curve, curveStartMS, frameMS)
	}
	if cfg.TargetF0 != nil {
		*cfg.TargetF0 = base.F0Track{StartMS: curveStartMS, FrameMS: frameMS, Hz: append([]float64(nil), f0Curve...)}
	}
	return worldlinePitchResult{
		sampleRate: sampleRate, pitches: pitches, intonation: intonation,
		pitchFactors: pitchFactors, reference: reference, f0Curve: f0Curve,
	}, nil
}

func normalizeWorldlineSources(synthesisPlan *plan.Plan, cache *base.SourceCache, sampleRate int, tempDir string) (map[string]string, error) {
	normalizedSources := make(map[string]string)
	for index := range synthesisPlan.Units {
		unit := &synthesisPlan.Units[index]
		if unit.Silent {
			continue
		}
		if _, ok := normalizedSources[unit.Source]; ok {
			continue
		}
		mono, err := cache.LoadMono(unit.Source)
		if err != nil {
			return nil, fmt.Errorf("read unit %q: %w", unit.Alias, err)
		}
		if mono.SampleRate == sampleRate {
			continue
		}
		resampled, err := cache.LoadNormalized(unit.Source, sampleRate)
		if err != nil {
			return nil, fmt.Errorf("normalize unit %q to %d Hz: %w", unit.Alias, sampleRate, err)
		}
		tempPath := filepath.Join(tempDir, fmt.Sprintf("resampled-%d.wav", index))
		if err := audio.WriteWav(tempPath, resampled); err != nil {
			return nil, fmt.Errorf("write resampled unit %q: %w", unit.Alias, err)
		}
		normalizedSources[unit.Source] = tempPath
	}
	return normalizedSources, nil
}

type worldlineUnitBuilder struct {
	plan              *plan.Plan
	cfg               base.Config
	cache             *base.SourceCache
	timing            worldlineTimingResult
	pitch             worldlinePitchResult
	normalizedSources map[string]string
	legacyMix         bool
}

func (b worldlineUnitBuilder) build(i int) (worldrender.Unit, error) {
	synthesisPlan, cfg := b.plan, b.cfg
	unit := &synthesisPlan.Units[i]
	timing := b.timing.units[i]
	phoneTiming := b.timing.phones[i]
	leadingMS := b.timing.leadingMS
	unitPitch := b.pitch.pitches[i]
	if unitPitch <= 0 {
		unitPitch = b.pitch.reference
	}
	unit.SourceF0Hz = b.pitch.pitches[i]
	unit.TargetF0Hz = unitPitch * b.pitch.pitchFactors[i] * base.PitchCurveFactorAt(cfg.PitchCurve, unit.NoteStartMS)
	unit.IntonationFactor = b.pitch.intonation[i]
	singleCVUnit := synthesisPlan.SingleCV && unit.Role == "mora"
	vcvUnit := unit.Role == "mora" && base.IsVCVUnit(*unit)
	volume := 100.0
	if unit.Role == "transition" {
		volume *= cfg.CVVCTransitionGain
	}
	if unit.ResamplerVolumeOverride {
		volume = float64(unit.ResamplerVolume)
	}

	// OpenUtauと同じピッチ開始位置を使い、先頭の余剰は除く。
	pitchLeadingMS := unit.PreutteranceMS
	if singleCVUnit {
		pitchLeadingMS = phoneTiming.Preutter
	}
	skipMS := math.Max(0, pitchLeadingMS-phoneTiming.Preutter)
	durCorrection := phoneTiming.Preutter - phoneTiming.TailIntrude + phoneTiming.TailOverlap
	envelopePoints := base.OpenUtauEnvelopeFromTiming(*unit, phoneTiming)
	if cfg.CVVCPreBoundaryFade && unit.Role == "transition" {
		envelopePoints = base.CVVCPreBoundaryEnvelope(envelopePoints, phoneTiming)
	}
	positionMS := unit.NoteStartMS - phoneTiming.Preutter + leadingMS
	consonantLength := unit.ConsonantMS
	if singleCVUnit {
		consonantLength = timing.ConsonantMS
	}
	requiredLength := math.Max(math.Max(unit.DurationMS+durCorrection, envelopePoints[4].XMS+phoneTiming.Preutter)+skipMS, consonantLength)
	requiredLength = math.Ceil(requiredLength/50+0.5) * 50
	if cfg.ProviderOptions.Worldline.ExactLength {
		requiredLength = unit.DurationMS
	}
	lengthMS := timing.PreutteranceMS + unit.DurationMS + cfg.ReleaseMS
	consonantVelocity := 100.0
	if positionMS < 0 {
		leadingTrimMS := -positionMS
		skipMS += leadingTrimMS
		lengthMS -= leadingTrimMS
		positionMS = 0
	}
	originalSource := unit.Source
	source, frqPath := originalSource, findFRQPath(originalSource)
	if normalized, ok := b.normalizedSources[unit.Source]; ok {
		source = normalized
		frqPath = ""
	}
	fadeInMS := math.Max(2, timing.PreutteranceMS-timing.OverlapMS)
	fadeOutMS := cfg.ReleaseMS
	if len(envelopePoints) == 5 {
		lengthMS = envelopePoints[4].XMS - envelopePoints[0].XMS
		fadeInMS = envelopePoints[1].XMS - envelopePoints[0].XMS
		fadeOutMS = envelopePoints[4].XMS - envelopePoints[3].XMS
	}
	codaRelease := worldCodaReleaseEligible(synthesisPlan, *unit)
	if codaRelease {
		envelopePoints, fadeOutMS = codaReleaseEnvelope(*unit, envelopePoints, fadeOutMS)
		if closure, release, ok := worldCodaReleaseSplit(synthesisPlan, *unit, cfg.ProviderOptions.Worldline); ok {
			unit.CodaClosureMS = closure
			unit.CodaReleaseMS = release
			unit.CodaReleaseSeparated = true
		}
	}
	// 解析キャッシュは変換前の原音と既定の音量で引く。
	cacheKey := worldlineAnalysisCacheKey(originalSource, frqPath, *unit)
	cacheKey += fmt.Sprintf("|fs=%d", b.pitch.sampleRate)
	strategy := b.resolveWorldlineSpeechStrategy(i, codaRelease, singleCVUnit, vcvUnit)
	speech := b.speechTiming(i, timing, skipMS, positionMS, strategy)
	gapRepair := b.legacyMix && worldlineGapRepairEligible(synthesisPlan, i)
	if b.legacyMix {
		unit.WorldRenderMode = plan.WorldRenderModeV13Compatible
		unit.WorldRenderReason = "japanese-continuous-low-processing"
	}
	unit.WorldGapRepairEligible = gapRepair
	if gapRepair {
		unit.WorldGapRepairReason = "same-vowel-voiced-boundary"
	}
	item := worldrender.Unit{
		Speech: speech, LegacyMix: b.legacyMix, GapRepair: gapRepair,
		CacheKey: cacheKey,
		Source:   source, FRQPath: frqPath, PositionMS: positionMS, SkipMS: skipMS,
		LengthMS: lengthMS, FadeInMS: fadeInMS,
		FadeOutMS: fadeOutMS, OffsetMS: unit.OffsetMS, RequiredLengthMS: requiredLength,
		ConsonantMS: unit.ConsonantMS, CutoffMS: unit.CutoffMS, ConsonantVelocity: consonantVelocity,
		Volume: volume, VolumeSet: unit.ResamplerVolumeOverride, EnergyFactor: unit.EnergyFactor,
		Envelope: worldEnvelope(envelopePoints),
	}
	if speech != nil && strategy.singleCVLegato {
		duration, err := b.sourceDurationMS(*unit)
		if err != nil {
			return worldrender.Unit{}, err
		}
		speech.Anchors = singleCVLegatoAnchors(speech.SourceOnsetMS, speech.TargetOnsetMS, requiredLength, duration)
	}
	return item, nil
}

type worldlineSpeechStrategy struct {
	codaRelease      bool
	singleCV         bool
	stopProtected    bool
	preserveStopOnly bool
	legacyE2BStop    bool
	stretch          bool
	singleCVLegato   bool
}

// resolveWorldlineSpeechStrategyはユニットの種類から方針を一度だけ決める。
func (b worldlineUnitBuilder) resolveWorldlineSpeechStrategy(i int, codaRelease, singleCVUnit, vcvUnit bool) worldlineSpeechStrategy {
	synthesisPlan, options := b.plan, b.cfg.ProviderOptions.Worldline
	unit := &synthesisPlan.Units[i]
	strategy := worldlineSpeechStrategy{
		codaRelease:    codaRelease,
		singleCV:       singleCVUnit,
		stopProtected:  worldlineStopProtection(synthesisPlan, *unit, options),
		singleCVLegato: singleCVLegato(synthesisPlan, *unit),
	}
	if base.SpeechStop(synthesisPlan, *unit) {
		if strategy.stopProtected {
			unit.StopBurstReason = "transient-detected"
		} else {
			unit.StopBurstReason = "transient-unreliable"
		}
	}
	// 日本語VCVは再伸縮せず、破裂音だけを保護する。
	strategy.preserveStopOnly = !b.legacyMix && unit.Role == "mora" && !singleCVUnit &&
		(!vcvUnit || e2bStopGeneralization(synthesisPlan, *unit, options)) && strategy.stopProtected
	strategy.legacyE2BStop = e2bLegacyStopPreserve(synthesisPlan, *unit, b.legacyMix, options) && !singleCVUnit
	// 固定部の補正をbridgeにも渡し、母音の伸びを抑える。
	strategy.stretch = unit.Role == "mora" && unit.StretchAdapted && !codaRelease
	return strategy
}

func (b worldlineUnitBuilder) speechTiming(i int, timing base.EffectiveTiming, skipMS, positionMS float64, strategy worldlineSpeechStrategy) *worldrender.SpeechTiming {
	synthesisPlan := b.plan
	unit := &synthesisPlan.Units[i]
	leadingMS := b.timing.leadingMS
	var speech *worldrender.SpeechTiming
	if unit.Role == "mora" && (strategy.singleCV || strategy.preserveStopOnly || strategy.legacyE2BStop || strategy.stretch) {
		targetOnset := skipMS + unit.NoteStartMS + leadingMS - positionMS
		if strategy.singleCV {
			targetOnset = timing.PreutteranceMS
		}
		speech = &worldrender.SpeechTiming{UnitIndex: i, SourceOnsetMS: speechSourceOnsetMS(*unit),
			TargetOnsetMS: targetOnset, ProtectStop: strategy.stopProtected, PreserveStopOnly: strategy.preserveStopOnly || strategy.legacyE2BStop}
		if unit.SpeechProfile != nil && unit.SpeechProfile.TransientConfidence >= stopTransientFloor {
			speech.SourceTransientMS = unit.SpeechProfile.TransientMS
			speech.SourceTransientDurationMS = unit.SpeechProfile.TransientDurationMS
		}
		if strategy.singleCV || strategy.stretch {
			speech.TargetFixedMS = timing.ConsonantMS
		}
		if i > 0 {
			if strategy.singleCV && base.SingleCVMoraBoundaryEligible(synthesisPlan, i) {
				speech.VowelJoin = !base.SingleCVProtectedOnset(synthesisPlan, *unit)
				speech.TargetJoinMS = timing.PreutteranceMS
				speech.TransitionLeftPhone = synthesisPlan.Morae[i-1].Vowel
				speech.TransitionRightPhone = base.SingleCVOnset(synthesisPlan, *unit)
				if speech.TransitionRightPhone == "" {
					speech.TransitionRightPhone = synthesisPlan.Morae[i].Vowel
				}
			} else {
				speech.VowelJoin = !synthesisPlan.Units[i-1].Silent && base.SpeechVowelJoin(synthesisPlan,
					base.RenderedUnit{Index: i - 1, Unit: synthesisPlan.Units[i-1]}, base.RenderedUnit{Index: i, Unit: *unit})
			}
		}
	}
	if strategy.codaRelease {
		speech = &worldrender.SpeechTiming{UnitIndex: i, SourceOnsetMS: unit.PreutteranceMS, TargetOnsetMS: skipMS + unit.NoteStartMS + leadingMS - positionMS, CodaRelease: true, ProtectStop: strategy.stopProtected}
		if unit.CodaReleaseSeparated {
			speech.SeparateRelease = true
			speech.ReleaseMS = unit.CodaReleaseMS
		}
		if unit.SpeechProfile != nil && unit.SpeechProfile.ReleaseTransientConfidence >= stopReleaseTransientFloor {
			speech.SourceTransientMS = unit.SpeechProfile.ReleaseTransientMS
			speech.SourceTransientDurationMS = unit.SpeechProfile.ReleaseTransientDurationMS
		}
	}
	return speech
}

// offsetから右ブランクまでの長さ(ms)。
func (b worldlineUnitBuilder) sourceDurationMS(unit plan.Unit) (float64, error) {
	mono, err := b.cache.LoadMono(unit.Source)
	if err != nil {
		return 0, err
	}
	sourceDuration := float64(len(mono.Data)/mono.Channels) * 1000 / float64(mono.SampleRate)
	return worldrender.SourceEndMS(sourceDuration, unit.OffsetMS, unit.CutoffMS) - unit.OffsetMS, nil
}

func renderWorldline(synthesisPlan *plan.Plan, cfg base.Config, phrase worldrender.Phrase, enginePath string, started *time.Time) (*audio.PCM, error) {
	ctx := cfg.Context
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	pcm, speechResults, err := renderWorld(ctx, phrase, enginePath)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("WORLD render canceled: %w", ctxErr)
		}
		return nil, fmt.Errorf("WORLD render failed: %w", err)
	}
	traceMark(started, "world")
	for _, result := range speechResults {
		if result.UnitIndex < 0 || result.UnitIndex >= len(synthesisPlan.Units) {
			return nil, fmt.Errorf("invalid WORLD speech report unit %d", result.UnitIndex)
		}
		unit := &synthesisPlan.Units[result.UnitIndex]
		unit.SpeechRetimeApplied = result.RetimeApplied
		unit.SpeechJoinApplied = result.JoinApplied
		unit.SpeechTransitionApplied = result.TransitionApplied
		unit.StopBurstApplied = result.StopBurstApplied
		unit.StopBurstGain = result.StopBurstGain
		if result.StopBurstApplied {
			unit.StopBurstReason = "transient-preserved"
		} else if unit.StopBurstReason == "transient-detected" {
			unit.StopBurstReason = "transient-outside-output"
		}
		if result.RetimeApplied {
			unit.EffectiveConsonantMS = result.TargetFixedMS
		}
	}
	return pcm, nil
}

func speechSourceOnsetMS(unit plan.Unit) float64 {
	profile := unit.SpeechProfile
	if profile == nil || profile.VoicingConfidence < .65 || profile.TransitionConfidence < .65 ||
		profile.VoicingStartMS <= 0 || math.Abs(profile.VoicingStartMS-unit.PreutteranceMS) > 25 {
		return unit.PreutteranceMS
	}
	return profile.VoicingStartMS
}

func legacyJapaneseContinuousMix(synthesisPlan *plan.Plan) bool {
	if synthesisPlan == nil || synthesisPlan.SingleCV {
		return false
	}
	return frontend.JapanesePlan(synthesisPlan.Language, synthesisPlan.Phonemizer)
}

// 低加工経路では同じ母音が直接続く境界だけを補間する。
func worldlineGapRepairEligible(synthesisPlan *plan.Plan, unitIndex int) bool {
	if synthesisPlan == nil || unitIndex <= 0 || unitIndex >= len(synthesisPlan.Units) {
		return false
	}
	previous, current := synthesisPlan.Units[unitIndex-1], synthesisPlan.Units[unitIndex]
	if previous.Silent || current.Silent || previous.Role != "mora" || current.Role != "mora" ||
		previous.Position+1 != current.Position || previous.Position < 0 || current.Position >= len(synthesisPlan.Morae) {
		return false
	}
	previousMora, currentMora := synthesisPlan.Morae[previous.Position], synthesisPlan.Morae[current.Position]
	return !previousMora.Pause && !currentMora.Pause && currentMora.Consonant == "" &&
		previousMora.Vowel != "" && previousMora.Vowel == currentMora.Vowel
}

func worldlineStopProtection(synthesisPlan *plan.Plan, unit plan.Unit, options base.WorldlineProviderOptions) bool {
	if synthesisPlan == nil {
		return false
	}
	if !base.SpeechStop(synthesisPlan, unit) {
		return false
	}
	if unit.SpeechProfile == nil {
		return false
	}
	if unit.Role == "ending" || len(unit.CodaPhones) > 0 {
		// 語末破裂音だけを、測定できた解放過渡の範囲で保護する。
		return base.CodaReleaseStop(unit) && unit.SpeechProfile.ReleaseTransientMS > 0 &&
			unit.SpeechProfile.ReleaseTransientConfidence >= stopReleaseTransientFloor
	}
	if unit.SpeechProfile.TransientConfidence < stopTransientFloor || unit.SpeechProfile.TransientMS <= 0 {
		return false
	}
	language := strings.ToLower(strings.TrimSpace(synthesisPlan.Language))
	phonemizer := strings.ToLower(strings.TrimSpace(synthesisPlan.Phonemizer))
	japanese := frontend.JapanesePlan(language, phonemizer)
	if japanese && strings.EqualFold(strings.TrimSpace(unit.AliasKind), "VCV") {
		return unit.SpeechProfile.TransientConfidence >= stopTransientVCVFloor
	}
	if japanese {
		return unit.SpeechProfile.TransientConfidence >= stopTransientJapaneseFloor
	}
	return true
}

func e2bStopGeneralization(synthesisPlan *plan.Plan, unit plan.Unit, options base.WorldlineProviderOptions) bool {
	if synthesisPlan == nil || unit.Silent || unit.Role != "mora" {
		return false
	}
	return frontend.JapanesePlan(synthesisPlan.Language, synthesisPlan.Phonemizer)
}

// 低加工の連続音でも、信頼度の高い破裂音だけは補う。
func e2bLegacyStopPreserve(synthesisPlan *plan.Plan, unit plan.Unit, legacyMix bool, options base.WorldlineProviderOptions) bool {
	if !legacyMix || !e2bStopGeneralization(synthesisPlan, unit, options) {
		return false
	}
	return worldlineStopProtection(synthesisPlan, unit, options)
}

// VCVはoto.iniの境界を使い、壊れた境界だけを補正する。
func worldlineTiming(synthesisPlan *plan.Plan, unit plan.Unit, releaseMS float64) base.EffectiveTiming {
	return base.NormalizePlanTiming(synthesisPlan, unit, releaseMS)
}

func worldEnvelope(points []base.WorldlineEnvelopePoint) []worldrender.EnvelopePoint {
	envelope := make([]worldrender.EnvelopePoint, len(points))
	for index, point := range points {
		envelope[index] = worldrender.EnvelopePoint{XMS: point.XMS, Y: point.Y}
	}
	return envelope
}

// CVVCの子音長にはVCを含める。タイムラインは本体で計算する。
func timingWarpJob(synthesisPlan *plan.Plan, cfg base.Config, frames int) *worldrender.TimingWarp {
	if synthesisPlan == nil || !cfg.ProviderOptions.Worldline.TimingWarpEnabled() {
		return nil
	}
	japanese := isJapanesePlan(synthesisPlan)
	transition := map[int]float64{}
	for _, unit := range synthesisPlan.Units {
		if unit.Role == "transition" && !unit.Silent {
			transition[unit.Position] = unit.DurationMS
		}
	}
	margin := synthesisPlan.LeadingMarginMS
	var morae []speechtiming.Mora
	for _, unit := range synthesisPlan.Units {
		if unit.Role != "mora" || unit.Silent || unit.Mora == "" {
			continue
		}
		entry := speechtiming.Mora{
			Text: unit.Mora, NoteStartMS: unit.NoteStartMS, DurationMS: unit.DurationMS,
			EffectivePreutteranceMS: math.Max(unit.EffectivePreutteranceMS, transition[unit.Position]),
		}
		// 日本語以外はかな解析できないため、プランの音素区間を余白込みで渡す。
		if !japanese {
			for _, timing := range synthesisPlan.PhoneTimings {
				if timing.Position != unit.Position {
					continue
				}
				entry.Spans = append(entry.Spans, speechtiming.Span{
					Label: timing.Symbol,
					Start: (margin + timing.StartMS) / 1000,
					End:   (margin + timing.StartMS + timing.DurationMS) / 1000,
				})
			}
		}
		morae = append(morae, entry)
	}
	timeline := speechtiming.PhoneTimeline(morae, margin, frames)
	spans := make([]worldrender.TimingWarpSpan, 0, len(timeline.Spans))
	for _, span := range timeline.Spans {
		spans = append(spans, worldrender.TimingWarpSpan{Label: span.Label, Start: span.Start, End: span.End})
	}
	return &worldrender.TimingWarp{Strength: 1, Language: synthesisPlan.Language, Spans: spans, Starts: timeline.Starts, Ends: timeline.Ends}
}

func isJapanesePlan(synthesisPlan *plan.Plan) bool {
	return frontend.NormalizeLanguage(synthesisPlan.Language) == "ja"
}

func findFRQPath(wavPath string) string {
	extension := filepath.Ext(wavPath)
	candidates := []string{
		strings.TrimSuffix(wavPath, extension) + "_wav.frq",
		strings.TrimSuffix(wavPath, extension) + ".frq",
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func resolveWorldEngine(configured string) (string, error) {
	if isWasm() {
		return configured, nil
	}
	if configured == "" {
		return "", errors.New("UtauTTS WORLD engine was not found in runtime")
	}
	if _, err := os.Stat(configured); err != nil {
		return "", fmt.Errorf("UtauTTS WORLD engine %q: %w", configured, err)
	}
	return configured, nil
}

func worldlineAnalysisCacheKey(source, frqPath string, unit plan.Unit) string {
	identity := func(path string) string {
		if path == "" {
			return ""
		}
		info, err := os.Stat(path)
		if err != nil {
			return path
		}
		return fmt.Sprintf("%s:%d:%d", path, info.Size(), info.ModTime().UnixNano())
	}
	return fmt.Sprintf("%s|%s|%.6f|%.6f|100.000000|86",
		identity(source), identity(frqPath), unit.OffsetMS, unit.CutoffMS)
}

func worldlineF0CurveAtOffset(synthesisPlan *plan.Plan, pitches, factors []float64, reference float64, length int, frameMS, startMS float64) []float64 {
	curve := make([]float64, length)
	for frame := range curve {
		timeMS := startMS + float64(frame)*frameMS
		curve[frame] = base.F0AtTime(synthesisPlan, pitches, factors, reference, timeMS)
	}
	return curve
}
