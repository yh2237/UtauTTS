package render

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"utautts/internal/audio"
	"utautts/internal/engine"
	"utautts/internal/plan"
	"utautts/internal/provider"
	"utautts/internal/render/worldline"
)

const worldlineFrameMS = 10.0

type worldlineManifest struct {
	Engine          string                  `json:"engine,omitempty"`
	WorldEnginePath string                  `json:"world_engine_path,omitempty"`
	OutputPath      string                  `json:"output_path"`
	SampleRate      int                     `json:"sample_rate"`
	F0Curve         []float64               `json:"f0_curve"`
	Units           []worldlineManifestUnit `json:"units"`
}

func renderUtauTTSWorldPhrase(synthesisPlan *plan.Plan, cfg Config) (*audio.PCM, error) {
	return renderWorldlineEngine(synthesisPlan, cfg, "utautts-world-phrase")
}

type worldlineManifestUnit struct {
	Speech            *provider.WorldSpeechTiming `json:"speech,omitempty"`
	LegacyMix         bool                        `json:"legacy_mix,omitempty"`
	GapRepair         bool                        `json:"gap_repair,omitempty"`
	CacheKey          string                      `json:"cache_key,omitempty"`
	Source            string                      `json:"source"`
	FRQPath           string                      `json:"frq_path,omitempty"`
	PositionMS        float64                     `json:"position_ms"`
	SkipMS            float64                     `json:"skip_ms"`
	LengthMS          float64                     `json:"length_ms"`
	FadeInMS          float64                     `json:"fade_in_ms"`
	FadeOutMS         float64                     `json:"fade_out_ms"`
	OffsetMS          float64                     `json:"offset_ms"`
	RequiredLengthMS  float64                     `json:"required_length_ms"`
	ConsonantMS       float64                     `json:"consonant_ms"`
	CutoffMS          float64                     `json:"cutoff_ms"`
	Tone              int                         `json:"tone"`
	ConsonantVelocity float64                     `json:"consonant_velocity"`
	PitchStartMS      float64                     `json:"pitch_start_ms,omitempty"`
	PitchLengthMS     float64                     `json:"pitch_length_ms,omitempty"`
	Volume            float64                     `json:"volume,omitempty"`
	VolumeSet         bool                        `json:"volume_set,omitempty"`
	Modulation        float64                     `json:"modulation,omitempty"`
	Tempo             float64                     `json:"tempo,omitempty"`
	EnergyFactor      float64                     `json:"energy_factor,omitempty"`
	Envelope          []worldlineEnvelopePoint    `json:"envelope,omitempty"`
}

type worldlineEnvelopePoint struct {
	XMS float64 `json:"x_ms"`
	Y   float64 `json:"y"`
}

func renderWorldlineEngine(synthesisPlan *plan.Plan, cfg Config, providerID string) (*audio.PCM, error) {
	if synthesisPlan == nil || len(synthesisPlan.Units) == 0 {
		return nil, errors.New("empty synthesis plan")
	}
	if cfg.CVVCTiming == "" {
		cfg.CVVCTiming = CVVCTimingSequential
	}
	if cfg.CVVCTiming != CVVCTimingSequential {
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
	worldEnginePath, err := resolveWorldEngine(cfg.resource(engine.ResourceWorldEngine))
	if err != nil {
		return nil, err
	}
	bridge, err := resolveWorldlineBridge(cfg.resource(engine.ResourceWorldlineBridge))
	if err != nil {
		return nil, err
	}
	cache := newSourceCache()
	timings := make([]effectiveTiming, len(synthesisPlan.Units))
	var phoneTimings []openUtauPhoneTiming
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
				phoneUnits[index].OverlapMS = singleCVWorldOverlapMS(synthesisPlan, phoneUnits[index], phoneUnits[index].PreutteranceMS)
			}
		}
		phoneTimings, phraseStartMS = openUtauPhoneTimingsWithCoda(phoneUnits, cfg.CVVCTiming, true)
	}
	leadingMS := limitLeadingPreutterance(math.Max(0, -phraseStartMS), cfg.LeadingPreutteranceMS)
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
		unit.WorldRenderMode = "adaptive"
		unit.WorldRenderReason = "adaptive-default"
		unit.WorldGapRepairEligible = false
		unit.WorldGapRepairReason = "not-required"
		vcvUnit := unit.Role == "mora" && isVCVUnit(*unit)
		vcvSpeech := vcvUnit && synthesisPlan.SpeechTiming
		timings[i] = worldlineTiming(synthesisPlan, *unit, cfg.ReleaseMS)
		timings[i] = adaptStretchTiming(*unit, timings[i], cfg.ReleaseMS, cfg.StretchAdapt, cfg.StretchAdaptStrength)
		if len(phoneTimings) == len(synthesisPlan.Units) && !unit.Silent {
			timings[i].preutteranceMS = phoneTimings[i].preutter
			timings[i].overlapMS = phoneTimings[i].overlap
			unit.CodaBoundaryLimited = phoneTimings[i].codaLimited
			// C3aでfixed境界をずらしたユニットはotoの値を上書きしない。
			if (unit.Role != "mora" || (!synthesisPlan.SingleCV && (!vcvUnit || !vcvSpeech))) && !timings[i].stretchAdapted {
				timings[i].consonantMS = unit.ConsonantMS
				timings[i].scale = 1
			}
		}
		unit.TimingScale = timings[i].scale
		unit.EffectivePreutteranceMS = timings[i].preutteranceMS
		unit.EffectiveConsonantMS = timings[i].consonantMS
		unit.EffectiveOverlapMS = timings[i].overlapMS
		unit.CVTimingApplied = timings[i].cvApplied
		unit.CVTimingWarnings = append([]string(nil), timings[i].cvWarnings...)
		unit.StretchAdapted = timings[i].stretchAdapted
		unit.StretchLimitReason = timings[i].stretchLimitReason
		unit.IntonationFactor = 1
	}
	intonation := identityFactors(len(synthesisPlan.Units))
	pitches, sampleRate, err := measureWorldlinePitches(synthesisPlan, &cache)
	if err != nil {
		return nil, err
	}
	if cfg.ApplyPitch {
		intonation = analyzeIntonationFromPitches(synthesisPlan, timings, pitches, cfg.IntonationStrength)
	}
	reference := medianFloat(nonzeroFloats(pitches))
	if reference <= 0 {
		reference = 220
	}

	pitchFactors := make([]float64, len(synthesisPlan.Units))
	for i, unit := range synthesisPlan.Units {
		pitchFactors[i] = intonation[i]
		pitchFactors[i] *= effectiveUnitPitchFactor(unit, cfg.ApplyPitch)
	}
	if cfg.ProviderOptions.Worldline.SpeechPitchReference && cfg.ApplyPitch {
		pitchFactors, reference = speechReferencePitchFactors(synthesisPlan, pitches, reference)
		for i, unit := range synthesisPlan.Units {
			intonation[i] = pitchFactors[i] / effectiveUnitPitchFactor(unit, true)
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
		manifest.F0Curve[frame] *= pitchCurveFactorAt(cfg.PitchCurve, curveStartMS+float64(frame)*frameMS)
	}
	if cfg.targetF0 != nil {
		*cfg.targetF0 = F0Track{StartMS: curveStartMS, FrameMS: frameMS, Hz: append([]float64(nil), manifest.F0Curve...)}
	}
	tempDir, err := os.MkdirTemp("", "utautts-worldline-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	// bridgeへ渡す前にサンプルレートを揃える。
	normalizedSources := make(map[string]string)
	for index := range synthesisPlan.Units {
		unit := &synthesisPlan.Units[index]
		if unit.Silent {
			continue
		}
		mono, err := cache.loadMono(unit.Source)
		if err != nil {
			return nil, fmt.Errorf("read unit %q: %w", unit.Alias, err)
		}
		if mono.SampleRate == sampleRate {
			continue
		}
		resampled, err := cache.loadNormalized(unit.Source, sampleRate)
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
		unit.TargetF0Hz = unitPitch * pitchFactors[i] * pitchCurveFactorAt(cfg.PitchCurve, unit.NoteStartMS)
		unit.IntonationFactor = intonation[i]
		consonantVelocity := 100.0
		if timing.consonantMS > 0 && unit.ConsonantMS > 0 {
			consonantVelocity = 100 * (1 + math.Log2(unit.ConsonantMS/timing.consonantMS))
		}
		requiredLength := timing.preutteranceMS + unit.DurationMS + cfg.ReleaseMS
		positionMS := unit.NoteStartMS - timing.preutteranceMS
		skipMS := 0.0
		lengthMS := requiredLength
		pitchStartMS := positionMS
		singleCVUnit := synthesisPlan.SingleCV && unit.Role == "mora"
		vcvUnit := unit.Role == "mora" && isVCVUnit(*unit)
		vcvSpeech := vcvUnit && synthesisPlan.SpeechTiming
		volume, modulation, tempo := 100.0, 0.0, 120.0
		if unit.Role == "transition" {
			volume *= cfg.CVVCTransitionGain
		}
		if unit.ResamplerVolumeOverride {
			volume = float64(unit.ResamplerVolume)
		}
		var envelopePoints []worldlineEnvelopePoint
		pitchLengthMS := 0.0
		if phraseTiming {
			// OpenUTAUと同じ位置からbendを始め、先頭の余剰をskipする。
			pitchLeadingMS := unit.PreutteranceMS
			if singleCVUnit || vcvSpeech {
				pitchLeadingMS = phoneTimings[i].preutter
			}
			skipMS = math.Max(0, pitchLeadingMS-timing.preutteranceMS)
			pitchStartMS = unit.NoteStartMS - pitchLeadingMS
			durCorrection := 0.0
			if phraseTiming {
				phoneTiming := phoneTimings[i]
				skipMS = math.Max(0, pitchLeadingMS-phoneTiming.preutter)
				durCorrection = phoneTiming.preutter - phoneTiming.tailIntrude + phoneTiming.tailOverlap
				envelopePoints = openUtauEnvelopeFromTiming(*unit, phoneTiming)
				if cfg.CVVCPreBoundaryFade && unit.Role == "transition" {
					envelopePoints = cvvcPreBoundaryEnvelope(envelopePoints, phoneTiming)
				}
				pitchLengthMS = envelopePoints[4].XMS + pitchLeadingMS
				if synthesisPlan.WordBoundaryEnvelope {
					envelopePoints, unit.BoundaryEnvelope = wordBoundaryEnvelope(synthesisPlan, *unit, envelopePoints)
				}
				positionMS = unit.NoteStartMS - phoneTiming.preutter + leadingMS
			}
			consonantLength := unit.ConsonantMS
			if singleCVUnit || vcvSpeech {
				consonantLength = timing.consonantMS
			}
			requiredLength = math.Max(unit.DurationMS+durCorrection+skipMS, consonantLength)
			requiredLength = math.Ceil(requiredLength/50+0.5) * 50
			if cfg.ProviderOptions.Worldline.ExactLength {
				requiredLength = unit.DurationMS
			}
			lengthMS = timing.preutteranceMS + unit.DurationMS + cfg.ReleaseMS
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
		fadeInMS := math.Max(2, timing.preutteranceMS-timing.overlapMS)
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
		if speechStop(synthesisPlan, *unit) {
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
				targetOnset = timing.preutteranceMS
			}
			speech = &provider.WorldSpeechTiming{UnitIndex: i, SourceOnsetMS: speechSourceOnsetMS(*unit),
				TargetOnsetMS: targetOnset, ProtectStop: stopProtected, PreserveStopOnly: protectStopOnly || legacyE2BStop}
			if unit.SpeechProfile != nil && unit.SpeechProfile.TransientConfidence >= stopTransientFloor {
				speech.SourceTransientMS = unit.SpeechProfile.TransientMS
				speech.SourceTransientDurationMS = unit.SpeechProfile.TransientDurationMS
			}
			if singleCVUnit || vcvSpeech || stretchSpeech {
				speech.TargetFixedMS = timing.consonantMS
			}
			if i > 0 {
				if singleCVUnit && singleCVMoraBoundaryEligible(synthesisPlan, i) {
					speech.VowelJoin = !singleCVProtectedOnset(synthesisPlan, *unit)
					speech.TargetJoinMS = timing.preutteranceMS
					speech.TransitionLeftPhone = synthesisPlan.Morae[i-1].Vowel
					speech.TransitionRightPhone = singleCVOnset(synthesisPlan, *unit)
					if speech.TransitionRightPhone == "" {
						speech.TransitionRightPhone = synthesisPlan.Morae[i].Vowel
					}
				} else {
					speech.VowelJoin = !synthesisPlan.Units[i-1].Silent && speechVowelJoin(synthesisPlan,
						renderedUnit{index: i - 1, unit: synthesisPlan.Units[i-1]}, renderedUnit{index: i, unit: *unit})
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
			unit.WorldRenderMode = "v1.3-compatible"
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
	if commandErr := worldline.InvokeReport(ctx, bridge, jobPath, manifest.OutputPath, &speechResults); commandErr != nil {
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
	minimumFrames := msToFrames(synthesisPlan.DurationMS+cfg.ReleaseMS+leadingMS, pcm.SampleRate)
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
func worldlineStopProtection(synthesisPlan *plan.Plan, unit plan.Unit, options WorldlineProviderOptions) bool {
	if synthesisPlan == nil {
		return false
	}
	if !speechStop(synthesisPlan, unit) {
		return false
	}
	if unit.SpeechProfile == nil {
		return false
	}
	if unit.Role == "ending" || len(unit.CodaPhones) > 0 {
		// 語末破裂音だけを、測定できた解放過渡の範囲で保護する。
		return codaReleaseStop(unit) && unit.SpeechProfile.ReleaseTransientMS > 0 &&
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
func e2bStopGeneralization(synthesisPlan *plan.Plan, unit plan.Unit, options WorldlineProviderOptions) bool {
	if !options.E2BEnabled() || synthesisPlan == nil || unit.Silent || unit.Role != "mora" {
		return false
	}
	language := strings.ToLower(strings.TrimSpace(synthesisPlan.Language))
	phonemizer := strings.ToLower(strings.TrimSpace(synthesisPlan.Phonemizer))
	return language == "ja" || phonemizer == "ja" || strings.HasPrefix(phonemizer, "ja-")
}

// e2bLegacyStopPreserveはレガシー日本語連続混合でも原波形バーストだけを重ねるかを返す。
// E2b無効時や信頼度が低いときは発動しない。
func e2bLegacyStopPreserve(synthesisPlan *plan.Plan, unit plan.Unit, legacyMix bool, options WorldlineProviderOptions) bool {
	if !legacyMix || !e2bStopGeneralization(synthesisPlan, unit, options) {
		return false
	}
	return worldlineStopProtection(synthesisPlan, unit, options)
}

// VCVはoto.iniの境界を使い、壊れた境界だけを補正する。
func worldlineTiming(synthesisPlan *plan.Plan, unit plan.Unit, releaseMS float64) effectiveTiming {
	timing := normalizeTiming(unit, releaseMS)
	if synthesisPlan == nil || unit.Silent || unit.Role != "mora" {
		return timing
	}
	if synthesisPlan.SingleCV {
		return normalizeSingleCVTiming(synthesisPlan, unit, timing, releaseMS)
	}
	if isVCVUnit(unit) {
		timing = normalizeVCVTiming(unit, timing, releaseMS)
	}
	timing.overlapMS = onsetOverlapMS(singleCVOnset(synthesisPlan, unit), timing.preutteranceMS, timing.overlapMS)
	return timing
}

// bridgeへ渡す音素時間を作る。
func worldlinePhoneTimingUnits(synthesisPlan *plan.Plan, releaseMS float64) []plan.Unit {
	if synthesisPlan == nil {
		return nil
	}
	needsCopy := synthesisPlan.SingleCV
	if !needsCopy {
		for _, unit := range synthesisPlan.Units {
			if isVCVUnit(unit) {
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
		if unit.Silent || unit.Role != "mora" || (!synthesisPlan.SingleCV && !isVCVUnit(unit)) {
			continue
		}
		timing := worldlineTiming(synthesisPlan, unit, releaseMS)
		result[index].PreutteranceMS = timing.preutteranceMS
		result[index].OverlapMS = timing.overlapMS
		result[index].ConsonantMS = timing.consonantMS
	}
	return result
}

func worldlineProviderJob(synthesisPlan *plan.Plan, cfg Config, manifest worldlineManifest, bridge string) (provider.UnitRendererJob, error) {
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
			PitchCurve:              providerPitchCurve(cfg.PitchCurve),
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

type openUtauPhoneTiming struct {
	preutter    float64
	overlap     float64
	tailIntrude float64
	tailOverlap float64
	overlapped  bool
	codaLimited bool
}

func openUtauPhoneTimings(units []plan.Unit, cvvcTiming string) ([]openUtauPhoneTiming, float64) {
	return openUtauPhoneTimingsWithCoda(units, cvvcTiming, false)
}

func openUtauPhoneTimingsWithCoda(units []plan.Unit, cvvcTiming string, protectCoda bool) ([]openUtauPhoneTiming, float64) {
	result := make([]openUtauPhoneTiming, len(units))
	previous := -1
	first := -1
	for index, unit := range units {
		if unit.Silent {
			continue
		}
		if first < 0 && unit.Role != "transition" {
			first = index
		}
		autoPreutter := unit.PreutteranceMS
		autoOverlap := unit.OverlapMS
		adjacent := false
		codaLimited := false
		if previous >= 0 {
			previousUnit := units[previous]
			gapMS := unit.NoteStartMS - (previousUnit.NoteStartMS + previousUnit.DurationMS)
			previousDuration := previousUnit.DurationMS
			maxPreutter := autoPreutter
			if gapMS <= 0 {
				adjacent = true
				if autoOverlap > 0 && autoPreutter-autoOverlap > previousDuration*0.5 {
					maxPreutter = previousDuration * 0.5 / (autoPreutter - autoOverlap) * autoPreutter
				} else if autoOverlap <= 0 {
					maxPreutter = math.Min(maxPreutter, previousDuration*0.9)
				}
				maxPreutter = math.Min(maxPreutter, previousDuration)
				if result[previous].preutter < 5 {
					maxPreutter = math.Min(maxPreutter, previousDuration+result[previous].preutter-5)
				}
			} else if gapMS < autoPreutter {
				maxPreutter = gapMS
			}
			if autoPreutter > maxPreutter && autoPreutter > 0 {
				ratio := maxPreutter / autoPreutter
				autoPreutter = maxPreutter
				autoOverlap *= ratio
			}
			if autoOverlap < 0 {
				autoOverlap = math.Max(autoOverlap, math.Min(0, 35-previousDuration+autoPreutter))
			}
			// 語末子音を持つ境界では次onsetの食い込みを制限し、coda末尾を残す。
			if protectCoda && adjacent && len(previousUnit.CodaPhones) > 0 {
				autoPreutter, autoOverlap, codaLimited = codaBoundaryOverlapMS(previousDuration, autoPreutter, autoOverlap)
			}
		}
		autoPreutter = math.Max(0, autoPreutter)
		result[index].preutter = autoPreutter
		result[index].overlap = autoOverlap
		result[index].overlapped = previous >= 0 && adjacent && autoOverlap > 0
		result[index].codaLimited = codaLimited
		if previous >= 0 {
			if adjacent {
				result[previous].tailIntrude = math.Max(result[previous].tailIntrude, math.Max(autoPreutter, autoPreutter-autoOverlap))
				result[previous].tailOverlap = math.Max(result[previous].tailOverlap, math.Max(autoOverlap, 0))
			}
		}
		if unit.Role != "transition" || cvvcTiming == CVVCTimingSequential {
			previous = index
		}
	}
	phraseStart := 0.0
	if first >= 0 {
		phraseStart = units[first].NoteStartMS - result[first].preutter
	}
	return result, phraseStart
}

func openUtauEnvelopeFromTiming(unit plan.Unit, timing openUtauPhoneTiming) []worldlineEnvelopePoint {
	fadeIn := 5.0
	if timing.overlapped {
		fadeIn = timing.overlap
	}
	fadeOut := 35.0
	if timing.tailOverlap > 0 {
		fadeOut = timing.tailOverlap
	}
	p0 := -timing.preutter
	p1 := math.Max(p0+5, p0+fadeIn)
	p2 := math.Max(0, p1)
	p4 := unit.DurationMS - timing.tailIntrude + timing.tailOverlap
	p3 := math.Max(p2, p4-fadeOut)
	return []worldlineEnvelopePoint{
		{XMS: p0, Y: 0}, {XMS: p1, Y: 1}, {XMS: p2, Y: 1},
		{XMS: p3, Y: 1}, {XMS: p4, Y: 0},
	}
}

func cvvcPreBoundaryEnvelope(points []worldlineEnvelopePoint, timing openUtauPhoneTiming) []worldlineEnvelopePoint {
	if len(points) != 5 {
		return points
	}
	result := append([]worldlineEnvelopePoint(nil), points...)
	fadeOut := math.Max(5, timing.tailOverlap)
	fadeStart := math.Max(result[1].XMS, -fadeOut)
	result[2].XMS = fadeStart
	result[3].XMS = fadeStart
	result[4].XMS = 0
	return result
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

func measureWorldlinePitches(synthesisPlan *plan.Plan, cache *sourceCache) ([]float64, int, error) {
	values := make([]float64, len(synthesisPlan.Units))
	sampleRate := 0
	type pitchJob struct {
		index int
		unit  plan.Unit
		mono  *audio.PCM
	}
	jobs := make([]pitchJob, 0, len(synthesisPlan.Units))
	for index, unit := range synthesisPlan.Units {
		if unit.Silent || unit.Role == "transition" {
			continue
		}
		mono, err := cache.loadMono(unit.Source)
		if err != nil {
			return nil, 0, err
		}
		if sampleRate == 0 {
			sampleRate = mono.SampleRate
		}
		jobs = append(jobs, pitchJob{index: index, unit: unit, mono: mono})
	}
	errs := make([]error, len(jobs))
	measure := func(jobIndex int) {
		job := jobs[jobIndex]
		values[job.index], errs[jobIndex] = estimateUnitPitch(job.unit, job.mono)
	}
	workers := min(len(jobs), max(1, runtime.GOMAXPROCS(0)))
	if workers <= 1 {
		for jobIndex := range jobs {
			measure(jobIndex)
		}
	} else {
		var group sync.WaitGroup
		group.Add(workers)
		for worker := 0; worker < workers; worker++ {
			go func(start int) {
				defer group.Done()
				for jobIndex := start; jobIndex < len(jobs); jobIndex += workers {
					measure(jobIndex)
				}
			}(worker)
		}
		group.Wait()
	}
	for _, err := range errs {
		if err != nil {
			return nil, 0, err
		}
	}
	if synthesisPlan.SingleCV {
		return stabilizeSingleCVPitches(synthesisPlan, values), sampleRate, nil
	}
	return stabilizeWorldlinePitches(values), sampleRate, nil
}

func stabilizeSingleCVPitches(synthesisPlan *plan.Plan, values []float64) []float64 {
	base := stabilizeWorldlinePitches(values)
	result := append([]float64(nil), base...)
	if synthesisPlan == nil || !synthesisPlan.SingleCV {
		return result
	}
	for index, value := range result {
		if value <= 0 || index >= len(synthesisPlan.Units) || synthesisPlan.Units[index].Silent || synthesisPlan.Units[index].Role == "transition" {
			continue
		}
		context := singleCVPitchContext(synthesisPlan, base, index)
		if len(context) < 2 {
			continue
		}
		if !singleCVPitchContextReliable(context) {
			continue
		}
		reference := medianFloat(context)
		if reference <= 0 {
			continue
		}
		ratio := value / reference
		if ratio >= .84 && ratio <= 1.19 {
			continue
		}
		best, bestDistance := value, math.Abs(math.Log2(ratio))
		for _, factor := range []float64{4.0 / 3, 3.0 / 2, 2, 3, 4, 3.0 / 4, 2.0 / 3, 1.0 / 2, 1.0 / 3, 1.0 / 4} {
			candidate := value * factor
			candidateRatio := candidate / reference
			if candidateRatio < .90 || candidateRatio > 1.10 {
				continue
			}
			distance := math.Abs(math.Log2(candidateRatio))
			if distance < bestDistance {
				best, bestDistance = candidate, distance
			}
		}
		if best == value {
			best = reference
		}
		result[index] = best
	}
	return result
}

func singleCVPitchContextReliable(values []float64) bool {
	if len(values) < 2 {
		return false
	}
	minimum, maximum := values[0], values[0]
	for _, value := range values[1:] {
		minimum = math.Min(minimum, value)
		maximum = math.Max(maximum, value)
	}
	return minimum > 0 && maximum/minimum <= 1.20
}

func singleCVPitchContext(synthesisPlan *plan.Plan, values []float64, index int) []float64 {
	result := make([]float64, 0, 5)
	for distance := 1; distance <= 2; distance++ {
		for _, neighbor := range []int{index - distance, index + distance} {
			if neighbor < 0 || neighbor >= len(values) || neighbor >= len(synthesisPlan.Units) {
				continue
			}
			unit := synthesisPlan.Units[neighbor]
			if unit.Silent || unit.Role == "transition" || unit.Position < 0 || unit.Position >= len(synthesisPlan.Morae) || values[neighbor] <= 0 {
				continue
			}
			if neighbor != index && math.Abs(float64(unit.Position-synthesisPlan.Units[index].Position)) > float64(distance) {
				continue
			}
			result = append(result, values[neighbor])
		}
	}
	return result
}

// 短い有声録音の倍音と分周誤検出を近い原音の高さへ補正する。
func stabilizeWorldlinePitches(values []float64) []float64 {
	result := append([]float64(nil), values...)
	reference := medianFloat(nonzeroFloats(values))
	for index, value := range values {
		if value <= 0 {
			continue
		}
		if reference > 0 && value < reference*.67 {
			best, bestDistance := value, math.Inf(1)
			for _, factor := range []float64{2, 3, 4} {
				candidate := value * factor
				ratio := candidate / reference
				if ratio < .87 || ratio > 1.15 {
					continue
				}
				if distance := math.Abs(math.Log2(ratio)); distance < bestDistance {
					best, bestDistance = candidate, distance
				}
			}
			result[index] = best
		}
	}
	for index, value := range values {
		if value <= 0 || result[index] != value {
			continue
		}
		neighbor := nearestWorldlinePitch(result, index)
		if neighbor <= 0 {
			continue
		}
		ratio := value / neighbor
		if ratio < 1.35 {
			continue
		}
		factor := 2.0 / 3.0
		if ratio >= 1.8 {
			factor = 0.5
		}
		correctedRatio := ratio * factor
		if correctedRatio >= 0.87 && correctedRatio <= 1.15 {
			result[index] = value * factor
		}
	}
	return result
}

func nearestWorldlinePitch(values []float64, index int) float64 {
	for distance := 1; distance < len(values); distance++ {
		left := index - distance
		if left >= 0 && values[left] > 0 {
			return values[left]
		}
		right := index + distance
		if right < len(values) && values[right] > 0 {
			return values[right]
		}
	}
	return 0
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

func nonzeroFloats(values []float64) []float64 {
	result := make([]float64, 0, len(values))
	for _, value := range values {
		if value > 0 {
			result = append(result, value)
		}
	}
	return result
}
