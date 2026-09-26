package worldline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"utautts/internal/audio"
	"utautts/internal/engine"
	"utautts/internal/plan"
	"utautts/internal/provider"
	"utautts/internal/render/base"
)

func init() {
	base.RegisterRenderer("utautts-world-phrase", renderUtauTTSWorldPhrase)
	base.RegisterCloser(func() error {
		Close()
		return nil
	})
}

const worldlineFrameMS = 10.0

type worldlineManifest struct {
	Engine          string                  `json:"engine,omitempty"`
	WorldEnginePath string                  `json:"world_engine_path,omitempty"`
	OutputPath      string                  `json:"output_path"`
	SampleRate      int                     `json:"sample_rate"`
	F0Curve         []float64               `json:"f0_curve"`
	Units           []worldlineManifestUnit `json:"units"`
}

func renderUtauTTSWorldPhrase(synthesisPlan *plan.Plan, cfg base.Config) (*audio.PCM, error) {
	return renderWorldlineEngine(synthesisPlan, cfg, "utautts-world-phrase")
}

type worldlineManifestUnit struct {
	Speech            *provider.WorldSpeechTiming   `json:"speech,omitempty"`
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

func renderWorldlineEngine(synthesisPlan *plan.Plan, cfg base.Config, providerID string) (*audio.PCM, error) {
	if synthesisPlan == nil || len(synthesisPlan.Units) == 0 {
		return nil, errors.New("empty synthesis plan")
	}
	if cfg.CVVCTiming == "" {
		cfg.CVVCTiming = base.CVVCTimingSequential
	}
	if cfg.CVVCTiming != base.CVVCTimingSequential {
		return nil, fmt.Errorf("unknown CVVC timing mode %q", cfg.CVVCTiming)
	}
	if cfg.CVVCTransitionGain == 0 {
		cfg.CVVCTransitionGain = 1
	}
	if cfg.CVVCTransitionGain < 0 || cfg.CVVCTransitionGain > 1 {
		return nil, fmt.Errorf("CVVC transition gain must be between 0 and 1; got %.3f", cfg.CVVCTransitionGain)
	}
	synthesisPlan.CVVCTiming = cfg.CVVCTiming
	synthesisPlan.CVVCTransitionGain = cfg.CVVCTransitionGain
	synthesisPlan.CVVCPreBoundaryFade = cfg.CVVCPreBoundaryFade
	worldEnginePath, err := resolveWorldEngine(cfg.Resource(engine.ResourceWorldEngine))
	if err != nil {
		return nil, err
	}
	bridge, err := resolveWorldlineBridge(cfg.Resource(engine.ResourceWorldlineBridge))
	if err != nil {
		return nil, err
	}
	cache := base.NewSourceCache()
	timings := make([]base.EffectiveTiming, len(synthesisPlan.Units))
	var phoneTimings []base.OpenUtauPhoneTiming
	phraseStartMS := 0.0
	phraseTiming := true
	legacyMix, err := worldlineLegacyMix(synthesisPlan, cfg.ProviderOptions.Worldline.MixMode)
	if err != nil {
		return nil, err
	}
	if phraseTiming {
		phoneUnits := worldlinePhoneTimingUnits(synthesisPlan, cfg.ReleaseMS)
		if synthesisPlan.SingleCV {
			for index := range phoneUnits {
				if phoneUnits[index].Silent || phoneUnits[index].Role != "mora" {
					continue
				}
				phoneUnits[index].OverlapMS = base.SingleCVWorldOverlapMS(synthesisPlan, phoneUnits[index], phoneUnits[index].PreutteranceMS)
			}
		}
		phoneTimings, phraseStartMS = base.OpenUtauPhoneTimingsWithCoda(phoneUnits, cfg.CVVCTiming, true)
	}
	leadingMS := base.LimitLeadingPreutterance(math.Max(0, -phraseStartMS), cfg.LeadingPreutteranceMS)
	synthesisPlan.LeadingMarginMS = leadingMS
	for i := range synthesisPlan.Units {
		unit := &synthesisPlan.Units[i]
		unit.SpeechRetimeApplied = false
		unit.BoundaryEnvelope = ""
		unit.SpeechJoinApplied = false
		unit.SpeechTransitionApplied = false
		unit.StopBurstApplied = false
		unit.StopBurstGain = 0
		unit.StopBurstReason = "not-required"
		unit.WorldRenderMode = plan.WorldRenderModeAdaptive
		unit.WorldRenderReason = "adaptive-default"
		unit.WorldGapRepairEligible = false
		unit.WorldGapRepairReason = "not-required"
		vcvUnit := unit.Role == "mora" && base.IsVCVUnit(*unit)
		vcvSpeech := vcvUnit && synthesisPlan.SpeechTiming
		timings[i] = worldlineTiming(synthesisPlan, *unit, cfg.ReleaseMS)
		timings[i] = base.AdaptStretchTiming(*unit, timings[i], cfg.ReleaseMS, cfg.StretchAdapt, cfg.StretchAdaptStrength)
		if len(phoneTimings) == len(synthesisPlan.Units) && !unit.Silent {
			timings[i].PreutteranceMS = phoneTimings[i].Preutter
			timings[i].OverlapMS = phoneTimings[i].Overlap
			unit.CodaBoundaryLimited = phoneTimings[i].CodaLimited
			// C3aでfixed境界をずらしたユニットはotoの値を上書きしない。
			if (unit.Role != "mora" || (!synthesisPlan.SingleCV && (!vcvUnit || !vcvSpeech))) && !timings[i].StretchAdapted {
				timings[i].ConsonantMS = unit.ConsonantMS
				timings[i].Scale = 1
			}
		}
		unit.TimingScale = timings[i].Scale
		unit.EffectivePreutteranceMS = timings[i].PreutteranceMS
		unit.EffectiveConsonantMS = timings[i].ConsonantMS
		unit.EffectiveOverlapMS = timings[i].OverlapMS
		unit.CVTimingApplied = timings[i].CVApplied
		unit.CVTimingWarnings = append([]string(nil), timings[i].CVWarnings...)
		unit.StretchAdapted = timings[i].StretchAdapted
		unit.StretchLimitReason = timings[i].StretchLimitReason
		unit.IntonationFactor = 1
	}
	intonation := base.IdentityFactors(len(synthesisPlan.Units))
	pitches, sampleRate, err := base.MeasureWorldlinePitches(synthesisPlan, &cache)
	if err != nil {
		return nil, err
	}
	if cfg.ApplyPitch {
		intonation = base.AnalyzeIntonationFromPitches(synthesisPlan, timings, pitches, cfg.IntonationStrength)
	}
	reference := base.MedianFloat(base.NonzeroFloats(pitches))
	if reference <= 0 {
		reference = 220
	}

	pitchFactors := make([]float64, len(synthesisPlan.Units))
	for i, unit := range synthesisPlan.Units {
		pitchFactors[i] = intonation[i]
		pitchFactors[i] *= base.EffectiveUnitPitchFactor(unit, cfg.ApplyPitch)
	}
	if cfg.ProviderOptions.Worldline.SpeechPitchReference && cfg.ApplyPitch {
		pitchFactors, reference = speechReferencePitchFactors(synthesisPlan, pitches, reference)
		for i, unit := range synthesisPlan.Units {
			intonation[i] = pitchFactors[i] / base.EffectiveUnitPitchFactor(unit, true)
		}
	}
	frameMS := worldlineFrameMS
	curveStartMS := 0.0
	curveDurationMS := synthesisPlan.DurationMS + cfg.ReleaseMS
	if phraseTiming {
		curveStartMS = -leadingMS
		curveDurationMS += leadingMS
	}
	f0Curve := worldlineF0CurveAtOffset(synthesisPlan, pitches, pitchFactors, reference,
		max(2, int(math.Ceil(curveDurationMS/frameMS))+2), frameMS, curveStartMS)
	manifest := worldlineManifest{
		Engine:          providerID,
		WorldEnginePath: worldEnginePath,
		SampleRate:      sampleRate,
		F0Curve:         f0Curve,
	}
	for frame := range manifest.F0Curve {
		manifest.F0Curve[frame] *= base.PitchCurveFactorAt(cfg.PitchCurve, curveStartMS+float64(frame)*frameMS)
	}
	if cfg.TargetF0 != nil {
		*cfg.TargetF0 = base.F0Track{StartMS: curveStartMS, FrameMS: frameMS, Hz: append([]float64(nil), manifest.F0Curve...)}
	}
	// bridgeへ渡す前にサンプルレートを揃える。一時ディレクトリはリサンプルが必要なときだけ作る。
	var tempDir string
	defer func() {
		if tempDir != "" {
			_ = os.RemoveAll(tempDir)
		}
	}()
	normalizedSources := make(map[string]string)
	for index := range synthesisPlan.Units {
		unit := &synthesisPlan.Units[index]
		if unit.Silent {
			continue
		}
		mono, err := cache.LoadMono(unit.Source)
		if err != nil {
			return nil, fmt.Errorf("read unit %q: %w", unit.Alias, err)
		}
		if mono.SampleRate == sampleRate {
			continue
		}
		if tempDir == "" {
			tempDir, err = os.MkdirTemp("", "utautts-worldline-")
			if err != nil {
				return nil, err
			}
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
	for i := range synthesisPlan.Units {
		unit := &synthesisPlan.Units[i]
		if unit.Silent {
			continue
		}
		timing := timings[i]
		unitPitch := pitches[i]
		if unitPitch <= 0 {
			unitPitch = reference
		}
		unit.SourceF0Hz = pitches[i]
		unit.TargetF0Hz = unitPitch * pitchFactors[i] * base.PitchCurveFactorAt(cfg.PitchCurve, unit.NoteStartMS)
		unit.IntonationFactor = intonation[i]
		consonantVelocity := 100.0
		if timing.ConsonantMS > 0 && unit.ConsonantMS > 0 {
			consonantVelocity = 100 * (1 + math.Log2(unit.ConsonantMS/timing.ConsonantMS))
		}
		requiredLength := timing.PreutteranceMS + unit.DurationMS + cfg.ReleaseMS
		positionMS := unit.NoteStartMS - timing.PreutteranceMS
		skipMS := 0.0
		lengthMS := requiredLength
		pitchStartMS := positionMS
		singleCVUnit := synthesisPlan.SingleCV && unit.Role == "mora"
		vcvUnit := unit.Role == "mora" && base.IsVCVUnit(*unit)
		vcvSpeech := vcvUnit && synthesisPlan.SpeechTiming
		volume, modulation, tempo := 100.0, 0.0, 120.0
		if unit.Role == "transition" {
			volume *= cfg.CVVCTransitionGain
		}
		if unit.ResamplerVolumeOverride {
			volume = float64(unit.ResamplerVolume)
		}
		var envelopePoints []base.WorldlineEnvelopePoint
		pitchLengthMS := 0.0
		if phraseTiming {
			// OpenUTAUと同じ位置からbendを始め、先頭の余剰をskipする。
			pitchLeadingMS := unit.PreutteranceMS
			if singleCVUnit || vcvSpeech {
				pitchLeadingMS = phoneTimings[i].Preutter
			}
			skipMS = math.Max(0, pitchLeadingMS-timing.PreutteranceMS)
			pitchStartMS = unit.NoteStartMS - pitchLeadingMS
			durCorrection := 0.0
			if phraseTiming {
				phoneTiming := phoneTimings[i]
				skipMS = math.Max(0, pitchLeadingMS-phoneTiming.Preutter)
				durCorrection = phoneTiming.Preutter - phoneTiming.TailIntrude + phoneTiming.TailOverlap
				envelopePoints = base.OpenUtauEnvelopeFromTiming(*unit, phoneTiming)
				if cfg.CVVCPreBoundaryFade && unit.Role == "transition" {
					envelopePoints = base.CVVCPreBoundaryEnvelope(envelopePoints, phoneTiming)
				}
				pitchLengthMS = envelopePoints[4].XMS + pitchLeadingMS
				if synthesisPlan.WordBoundaryEnvelope {
					envelopePoints, unit.BoundaryEnvelope = wordBoundaryEnvelope(synthesisPlan, *unit, envelopePoints)
				}
				positionMS = unit.NoteStartMS - phoneTiming.Preutter + leadingMS
			}
			consonantLength := unit.ConsonantMS
			if singleCVUnit || vcvSpeech {
				consonantLength = timing.ConsonantMS
			}
			requiredLength = math.Max(unit.DurationMS+durCorrection+skipMS, consonantLength)
			requiredLength = math.Ceil(requiredLength/50+0.5) * 50
			if cfg.ProviderOptions.Worldline.ExactLength {
				requiredLength = unit.DurationMS
			}
			lengthMS = timing.PreutteranceMS + unit.DurationMS + cfg.ReleaseMS
			consonantVelocity = 100
		}
		if positionMS < 0 {
			leadingTrimMS := -positionMS
			skipMS += leadingTrimMS
			lengthMS -= leadingTrimMS
			positionMS = 0
		}
		originalSource := unit.Source
		source, frqPath := originalSource, findFRQPath(originalSource)
		if normalized, ok := normalizedSources[unit.Source]; ok {
			source = normalized
			frqPath = ""
		}
		fadeInMS := math.Max(2, timing.PreutteranceMS-timing.OverlapMS)
		fadeOutMS := cfg.ReleaseMS
		if phraseTiming && len(envelopePoints) == 5 {
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
		cacheSource := source
		cacheVolume := volume
		cacheSource = originalSource
		cacheVolume = 100
		cacheKey := worldlineAnalysisCacheKey(cacheSource, frqPath, *unit, cacheVolume)
		cacheKey += fmt.Sprintf("|fs=%d", sampleRate)
		var speech *provider.WorldSpeechTiming
		stopProtected := worldlineStopProtection(synthesisPlan, *unit, cfg.ProviderOptions.Worldline)
		if base.SpeechStop(synthesisPlan, *unit) {
			if stopProtected {
				unit.StopBurstReason = "transient-detected"
			} else {
				unit.StopBurstReason = "transient-unreliable"
			}
		}
		// E2bは日本語VCVにも原波形バーストを広げるが、再伸縮はせずpreserve-onlyに留める。
		protectStopOnly := !legacyMix && unit.Role == "mora" && !singleCVUnit && !vcvSpeech &&
			(!vcvUnit || e2bStopGeneralization(synthesisPlan, *unit, cfg.ProviderOptions.Worldline)) && stopProtected
		legacyE2BStop := e2bLegacyStopPreserve(synthesisPlan, *unit, legacyMix, cfg.ProviderOptions.Worldline) && !singleCVUnit && !vcvSpeech
		// C3aで伸縮を有界にしたユニットは、bridge側でもfixed境界を後ろへずらして母音の伸びを抑える。
		stretchSpeech := unit.Role == "mora" && unit.StretchAdapted && !codaRelease
		if unit.Role == "mora" && (singleCVUnit || vcvSpeech || synthesisPlan.SpeechTiming && unit.SpeechProfile != nil && unit.SpeechProfile.Applied || protectStopOnly || legacyE2BStop || stretchSpeech) {
			targetOnset := skipMS + unit.NoteStartMS + leadingMS - positionMS
			if singleCVUnit || vcvSpeech {
				targetOnset = timing.PreutteranceMS
			}
			speech = &provider.WorldSpeechTiming{UnitIndex: i, SourceOnsetMS: speechSourceOnsetMS(*unit),
				TargetOnsetMS: targetOnset, ProtectStop: stopProtected, PreserveStopOnly: protectStopOnly || legacyE2BStop}
			if unit.SpeechProfile != nil && unit.SpeechProfile.TransientConfidence >= stopTransientFloor {
				speech.SourceTransientMS = unit.SpeechProfile.TransientMS
				speech.SourceTransientDurationMS = unit.SpeechProfile.TransientDurationMS
			}
			if singleCVUnit || vcvSpeech || stretchSpeech {
				speech.TargetFixedMS = timing.ConsonantMS
			}
			if i > 0 {
				if singleCVUnit && base.SingleCVMoraBoundaryEligible(synthesisPlan, i) {
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
		if codaRelease {
			speech = &provider.WorldSpeechTiming{UnitIndex: i, SourceOnsetMS: unit.PreutteranceMS, TargetOnsetMS: skipMS + unit.NoteStartMS + leadingMS - positionMS, CodaRelease: true, ProtectStop: stopProtected}
			if unit.CodaReleaseSeparated {
				speech.SeparateRelease = true
				speech.ReleaseMS = unit.CodaReleaseMS
			}
			if unit.SpeechProfile != nil && unit.SpeechProfile.ReleaseTransientConfidence >= stopReleaseTransientFloor {
				speech.SourceTransientMS = unit.SpeechProfile.ReleaseTransientMS
				speech.SourceTransientDurationMS = unit.SpeechProfile.ReleaseTransientDurationMS
			}
		}
		gapRepair, err := worldlineGapRepair(synthesisPlan, i, legacyMix, cfg.ProviderOptions.Worldline.GapRepairMode)
		if err != nil {
			return nil, err
		}
		if legacyMix {
			unit.WorldRenderMode = plan.WorldRenderModeV13Compatible
			unit.WorldRenderReason = "japanese-continuous-low-processing"
		}
		unit.WorldGapRepairEligible = gapRepair
		if gapRepair {
			unit.WorldGapRepairReason = "same-vowel-voiced-boundary"
		}
		manifest.Units = append(manifest.Units, worldlineManifestUnit{

			Speech: speech, LegacyMix: legacyMix, GapRepair: gapRepair,
			CacheKey: cacheKey,
			Source:   source, FRQPath: frqPath, PositionMS: positionMS, SkipMS: skipMS,
			LengthMS: lengthMS, FadeInMS: fadeInMS,
			FadeOutMS: fadeOutMS, OffsetMS: unit.OffsetMS, RequiredLengthMS: requiredLength,
			ConsonantMS: unit.ConsonantMS, CutoffMS: unit.CutoffMS,
			Tone: int(math.Round(69 + 12*math.Log2(unitPitch/440))), ConsonantVelocity: consonantVelocity,
			PitchStartMS: pitchStartMS, Volume: volume, VolumeSet: unit.ResamplerVolumeOverride,
			Modulation: modulation, Tempo: tempo,
			EnergyFactor:  unit.EnergyFactor,
			PitchLengthMS: pitchLengthMS, Envelope: envelopePoints,
		})
	}

	manifest.OutputPath = filepath.Join(tempDir, "output.wav")
	job, err := worldlineProviderJob(synthesisPlan, cfg, manifest, bridge)
	if err != nil {
		return nil, err
	}
	jobPath := filepath.Join(tempDir, "job.json")
	jobData, err := json.Marshal(job)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(jobPath, jobData, 0o600); err != nil {
		return nil, err
	}
	ctx := cfg.Context
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	var speechResults []provider.WorldSpeechResult
	if commandErr := InvokeReport(ctx, bridge, jobPath, manifest.OutputPath, &speechResults); commandErr != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("worldline bridge canceled: %w", ctxErr)
		}
		return nil, fmt.Errorf("worldline bridge failed: %w", commandErr)
	}
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
	pcm, err := audio.ReadWav(manifest.OutputPath)
	if err != nil {
		return nil, fmt.Errorf("read worldline output: %w", err)
	}
	minimumFrames := base.MsToFrames(synthesisPlan.DurationMS+cfg.ReleaseMS+leadingMS, pcm.SampleRate)
	if len(pcm.Data) < minimumFrames {
		pcm.Data = append(pcm.Data, make([]int16, minimumFrames-len(pcm.Data))...)
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
	if synthesisPlan == nil || synthesisPlan.SingleCV || synthesisPlan.SpeechTiming {
		return false
	}
	language := strings.ToLower(strings.TrimSpace(synthesisPlan.Language))
	phonemizer := strings.ToLower(strings.TrimSpace(synthesisPlan.Phonemizer))
	return language == "ja" || phonemizer == "ja" || strings.HasPrefix(phonemizer, "ja-")
}

func worldlineLegacyMix(synthesisPlan *plan.Plan, mode string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "auto":
		return legacyJapaneseContinuousMix(synthesisPlan), nil
	case "v1.3", "legacy":
		return true, nil
	case "adaptive":
		return false, nil
	default:
		return false, fmt.Errorf("unknown WORLD mix mode %q", mode)
	}
}

func worldlineGapRepair(synthesisPlan *plan.Plan, unitIndex int, legacyMix bool, mode string) (bool, error) {
	if !legacyMix {
		return false, nil
	}
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "auto":
		return worldlineGapRepairEligible(synthesisPlan, unitIndex), nil
	case "on":
		return unitIndex > 0, nil
	case "off":
		return false, nil
	default:
		return false, fmt.Errorf("unknown WORLD gap repair mode %q", mode)
	}
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

// 生波形補強は英語の破裂音と単独音に加え、E2bで日本語の破裂音にも広げる。
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
	japanese := language == "ja" || phonemizer == "ja" || strings.HasPrefix(phonemizer, "ja-")
	if japanese && strings.EqualFold(strings.TrimSpace(unit.AliasKind), "VCV") {
		// E2b: 日本語VCVは信頼度が高い過渡だけ保護する。
		return options.E2BEnabled() && unit.SpeechProfile.TransientConfidence >= stopTransientVCVFloor
	}
	if japanese {
		// E2b: 日本語CVも既定では無効。信頼度が高いときだけ保護する。
		return options.E2BEnabled() && unit.SpeechProfile.TransientConfidence >= stopTransientJapaneseFloor
	}
	return true
}

// e2bStopGeneralizationはE2bが対象とする日本語の破裂音モーラかを返す。
func e2bStopGeneralization(synthesisPlan *plan.Plan, unit plan.Unit, options base.WorldlineProviderOptions) bool {
	if !options.E2BEnabled() || synthesisPlan == nil || unit.Silent || unit.Role != "mora" {
		return false
	}
	language := strings.ToLower(strings.TrimSpace(synthesisPlan.Language))
	phonemizer := strings.ToLower(strings.TrimSpace(synthesisPlan.Phonemizer))
	return language == "ja" || phonemizer == "ja" || strings.HasPrefix(phonemizer, "ja-")
}

// e2bLegacyStopPreserveはレガシー日本語連続混合でも原波形バーストだけを重ねるかを返す。
// E2b無効時や信頼度が低いときは発動しない。
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

// bridgeへ渡す音素時間を作る。
func worldlinePhoneTimingUnits(synthesisPlan *plan.Plan, releaseMS float64) []plan.Unit {
	if synthesisPlan == nil {
		return nil
	}
	needsCopy := synthesisPlan.SingleCV
	if !needsCopy {
		for _, unit := range synthesisPlan.Units {
			if base.IsVCVUnit(unit) {
				needsCopy = true
				break
			}
		}
	}
	if !needsCopy {
		return synthesisPlan.Units
	}
	result := append([]plan.Unit(nil), synthesisPlan.Units...)
	for index := range result {
		unit := result[index]
		if unit.Silent || unit.Role != "mora" || (!synthesisPlan.SingleCV && !base.IsVCVUnit(unit)) {
			continue
		}
		timing := worldlineTiming(synthesisPlan, unit, releaseMS)
		result[index].PreutteranceMS = timing.PreutteranceMS
		result[index].OverlapMS = timing.OverlapMS
		result[index].ConsonantMS = timing.ConsonantMS
	}
	return result
}

func worldlineProviderJob(synthesisPlan *plan.Plan, cfg base.Config, manifest worldlineManifest, bridge string) (provider.UnitRendererJob, error) {
	planData, err := json.Marshal(plan.Clone(synthesisPlan))
	if err != nil {
		return provider.UnitRendererJob{}, err
	}
	resources := map[string]string{
		"world_engine":     manifest.WorldEnginePath,
		"worldline_bridge": bridge,
	}
	for key, value := range resources {
		if strings.TrimSpace(value) == "" {
			delete(resources, key)
		}
	}
	if len(resources) == 0 {
		resources = nil
	}
	worldline := provider.WorldlineOptions{
		Engine: manifest.Engine, SampleRate: manifest.SampleRate, ExactLength: cfg.ProviderOptions.Worldline.ExactLength,
		F0Curve: append([]float64(nil), manifest.F0Curve...),
		Units:   make([]provider.WorldlineUnit, len(manifest.Units)),
	}
	for index, unit := range manifest.Units {
		converted := provider.WorldlineUnit{
			Speech: unit.Speech, LegacyMix: unit.LegacyMix, GapRepair: unit.GapRepair,
			CacheKey: unit.CacheKey, Source: unit.Source, FRQPath: unit.FRQPath,
			PositionMS: unit.PositionMS, SkipMS: unit.SkipMS, LengthMS: unit.LengthMS,
			FadeInMS: unit.FadeInMS, FadeOutMS: unit.FadeOutMS, OffsetMS: unit.OffsetMS,
			RequiredLengthMS: unit.RequiredLengthMS, ConsonantMS: unit.ConsonantMS,
			CutoffMS: unit.CutoffMS, Tone: unit.Tone, ConsonantVelocity: unit.ConsonantVelocity,
			PitchStartMS: unit.PitchStartMS, PitchLengthMS: unit.PitchLengthMS,
			Volume: unit.Volume, VolumeSet: unit.VolumeSet,
			Modulation: unit.Modulation, Tempo: unit.Tempo, EnergyFactor: unit.EnergyFactor,
			Envelope: make([]provider.WorldlineEnvelopePoint, len(unit.Envelope)),
		}
		for pointIndex, point := range unit.Envelope {
			converted.Envelope[pointIndex] = provider.WorldlineEnvelopePoint{XMS: point.XMS, Y: point.Y}
		}
		worldline.Units[index] = converted
	}
	return provider.UnitRendererJob{
		Version:         provider.UnitRendererJobVersion,
		Contract:        "unit-renderer",
		ContractVersion: 1,
		Plan:            planData,
		Options: provider.UnitRendererOptions{
			ReleaseMS:               cfg.ReleaseMS,
			LeadingPreutteranceMS:   cfg.LeadingPreutteranceMS,
			IntonationStrength:      cfg.IntonationStrength,
			ApplyPitch:              cfg.ApplyPitch,
			BoundaryBridgeMS:        cfg.BoundaryBridgeMS,
			BoundaryBridgeThreshold: cfg.BoundaryBridgeThreshold,
			CVVCTiming:              cfg.CVVCTiming,
			CVVCTransitionGain:      cfg.CVVCTransitionGain,
			CVVCPreBoundaryFade:     cfg.CVVCPreBoundaryFade,
			PitchCurve:              base.ProviderPitchCurve(cfg.PitchCurve),
			Worldline:               &worldline,
		},
		Resources: resources,
	}, nil
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

func resolveWorldlineBridge(configured string) (string, error) {
	if configured == "" {
		return "", errors.New("worldline bridge is not configured by the renderer plugin")
	}
	if _, err := os.Stat(configured); err != nil {
		return "", fmt.Errorf("worldline bridge %q: %w", configured, err)
	}
	return configured, nil
}

func resolveWorldEngine(configured string) (string, error) {
	if configured == "" {
		return "", errors.New("UtauTTS WORLD engine is not configured by the renderer plugin")
	}
	if _, err := os.Stat(configured); err != nil {
		return "", fmt.Errorf("UtauTTS WORLD engine %q: %w", configured, err)
	}
	return configured, nil
}

func worldlineAnalysisCacheKey(source, frqPath string, unit plan.Unit, volume float64) string {
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
	return fmt.Sprintf("%s|%s|%.6f|%.6f|%.6f|86",
		identity(source), identity(frqPath), unit.OffsetMS, unit.CutoffMS, volume)
}

func worldlineF0Curve(synthesisPlan *plan.Plan, pitches, factors []float64, reference float64, length int) []float64 {
	return worldlineF0CurveAt(synthesisPlan, pitches, factors, reference, length, worldlineFrameMS)
}

func worldlineF0CurveAt(synthesisPlan *plan.Plan, pitches, factors []float64, reference float64, length int, frameMS float64) []float64 {
	return worldlineF0CurveAtOffset(synthesisPlan, pitches, factors, reference, length, frameMS, 0)
}

func worldlineF0CurveAtOffset(synthesisPlan *plan.Plan, pitches, factors []float64, reference float64, length int, frameMS, startMS float64) []float64 {
	targets := make([]float64, len(pitches))
	for i, value := range pitches {
		if value <= 0 {
			value = reference
		}
		targets[i] = value * factors[i]
	}
	curve := make([]float64, length)
	unitIndex := 0
	for frame := range curve {
		timeMS := startMS + float64(frame)*frameMS
		for unitIndex+1 < len(synthesisPlan.Units) && synthesisPlan.Units[unitIndex+1].NoteStartMS <= timeMS {
			unitIndex++
		}
		value := targets[unitIndex]
		if unitIndex+1 < len(targets) {
			left := synthesisPlan.Units[unitIndex].NoteStartMS
			right := synthesisPlan.Units[unitIndex+1].NoteStartMS
			if right > left {
				progress := math.Max(0, math.Min(1, (timeMS-left)/(right-left)))
				value = math.Exp(math.Log(targets[unitIndex])*(1-progress) + math.Log(targets[unitIndex+1])*progress)
			}
		}
		curve[frame] = value
	}
	return curve
}
