package render

import (
	"errors"
	"fmt"
	"math"
	"sync"

	"utautts/internal/audio"
	"utautts/internal/engine"
	"utautts/internal/plan"
	"utautts/internal/render/base"
)

func init() {
	base.RegisterRenderer("waveform", renderWaveform)
	base.RegisterRenderer("utau-external-resampler", renderUtauExternalResampler)
}

func IsKnownRenderer(id string) bool {
	if id == "" {
		return true
	}
	return engine.BuiltinRegistry().Supports(engine.ProviderID(id))
}

var boundaryBridgeRenderers = map[string]struct{}{
	"": {}, "waveform": {},
}

func Render(synthesisPlan *plan.Plan, cfg Config) (*audio.PCM, error) {
	return renderMutable(synthesisPlan, cfg)
}

func renderMutable(synthesisPlan *plan.Plan, cfg Config) (*audio.PCM, error) {
	if err := contextError(cfg.Context); err != nil {
		return nil, err
	}
	for name, value := range map[string]float64{
		"release_ms":                cfg.ReleaseMS,
		"leading_preutterance_ms":   cfg.LeadingPreutteranceMS,
		"intonation_strength":       cfg.IntonationStrength,
		"boundary_bridge_ms":        cfg.BoundaryBridgeMS,
		"boundary_bridge_threshold": cfg.BoundaryBridgeThreshold,
	} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, fmt.Errorf("%s must be finite, got %v", name, value)
		}
	}
	if cfg.ReleaseMS < 0 {
		return nil, fmt.Errorf("release_ms must be non-negative, got %v", cfg.ReleaseMS)
	}
	if cfg.LeadingPreutteranceMS < 0 {
		return nil, fmt.Errorf("leading_preutterance_ms must be non-negative, got %v", cfg.LeadingPreutteranceMS)
	}
	if !cfg.ReleaseSet && cfg.ReleaseMS == 0 {
		cfg.ReleaseMS = DefaultReleaseMS
	}
	if cfg.IntonationStrength < 0 || cfg.IntonationStrength > MaxIntonationStrength {
		return nil, fmt.Errorf("intonation_strength must be between 0 and %.0f, got %v", MaxIntonationStrength, cfg.IntonationStrength)
	}
	if !cfg.ApplyPitch {
		cfg.IntonationStrength = 0
		cfg.PitchCurve = nil
	}
	if cfg.PitchCurve != nil {
		if cfg.PitchCurve.FrameMS < 0.1 || math.IsNaN(cfg.PitchCurve.FrameMS) || math.IsInf(cfg.PitchCurve.FrameMS, 0) || len(cfg.PitchCurve.Cents) == 0 {
			return nil, errors.New("pitch curve requires frame_ms >= 0.1 and at least one value")
		}
		for index, cents := range cfg.PitchCurve.Cents {
			if math.IsNaN(cents) || math.IsInf(cents, 0) {
				return nil, fmt.Errorf("pitch curve value %d is not finite", index)
			}
			if math.Abs(cents) > 4800 {
				return nil, fmt.Errorf("pitch curve value %d is outside the supported +/-4800 cent range", index)
			}
		}
	}
	if cfg.BoundaryBridgeMS > 0 && !rendererSupportsBoundaryBridge(cfg.Backend) {
		return nil, fmt.Errorf("boundary bridge requires waveform renderer, got %q", cfg.Backend)
	}
	if cfg.Backend == "" {
		return renderWaveform(synthesisPlan, cfg)
	}
	if backend, ok := base.RendererImplementation(cfg.Backend); ok {
		return backend(synthesisPlan, cfg)
	}
	return nil, fmt.Errorf("unknown renderer backend %q", cfg.Backend)
}

func rendererSupportsBoundaryBridge(renderer string) bool {
	_, ok := boundaryBridgeRenderers[renderer]
	return ok
}

func renderWaveform(synthesisPlan *plan.Plan, cfg Config) (*audio.PCM, error) {
	return renderWaveformWithStretch(synthesisPlan, cfg, true, func(source []float64, targetFrames, sourcePrefixFrames, targetPrefixFrames, sampleRate int) ([]float64, error) {
		return retimeWithCompressedPrefixUsing(source, targetFrames, sourcePrefixFrames, targetPrefixFrames, sampleRate, wsolaStretch)
	})
}

const maxParallelRetimeUnits = 32

type preparedWaveformUnit struct {
	unitIndex                   int
	timing                      effectiveTiming
	wave                        []float64
	targetFrames                int
	sourceConsonantFrames       int
	sourcePreutteranceFrames    int
	speechSourceConsonantFrames int
	effectiveConsonantFrames    int
}

func renderWaveformWithStretch(synthesisPlan *plan.Plan, cfg Config, parallelRetime bool, retime func([]float64, int, int, int, int) ([]float64, error)) (*audio.PCM, error) {
	if synthesisPlan == nil || len(synthesisPlan.Units) == 0 {
		return nil, errors.New("empty synthesis plan")
	}
	synthesisPlan.BoundaryBridgeMS = 0
	synthesisPlan.BoundaryBridgeThreshold = 0
	synthesisPlan.BoundaryBridges = nil
	synthesisPlan.BoundaryRepairDecisions = nil
	if cfg.BoundaryBridgeMS > 0 {
		synthesisPlan.BoundaryBridgeMS = cfg.BoundaryBridgeMS
		synthesisPlan.BoundaryBridgeThreshold = cfg.BoundaryBridgeThreshold
	}

	cache := newSourceCache()
	sampleRate := 0
	var mix []float64
	var mixWeights []float64
	rendered := make([]renderedUnit, 0, len(synthesisPlan.Units))
	timings := make([]effectiveTiming, len(synthesisPlan.Units))
	for i := range synthesisPlan.Units {
		unit := &synthesisPlan.Units[i]
		unit.SpeechRetimeApplied = false
		unit.SpeechJoinApplied = false
		timings[i] = normalizePlanTiming(synthesisPlan, *unit, cfg.ReleaseMS)
		timings[i] = adaptStretchTiming(*unit, timings[i], cfg.ReleaseMS, cfg.StretchAdapt, cfg.StretchAdaptStrength)
		unit.TimingScale = timings[i].Scale
		unit.EffectivePreutteranceMS = timings[i].PreutteranceMS
		unit.EffectiveConsonantMS = timings[i].ConsonantMS
		unit.EffectiveOverlapMS = timings[i].PreutteranceMS - fadeInDurationMS(timings[i])
		unit.CVTimingApplied = timings[i].CVApplied
		unit.CVTimingWarnings = append([]string(nil), timings[i].CVWarnings...)
		unit.StretchAdapted = timings[i].StretchAdapted
		unit.StretchLimitReason = timings[i].StretchLimitReason
		unit.IntonationFactor = 1
	}
	leadingMS := limitLeadingPreutterance(leadingPreutteranceMS(synthesisPlan.Units, timings), cfg.LeadingPreutteranceMS)
	synthesisPlan.LeadingMarginMS = leadingMS
	intonation := identityFactors(len(synthesisPlan.Units))
	if cfg.ApplyPitch {
		intonation = analyzeIntonation(synthesisPlan, timings, &cache, cfg.IntonationStrength)
	}
	prepared := make([]preparedWaveformUnit, 0, len(synthesisPlan.Units))
	for unitIndex := range synthesisPlan.Units {
		if err := contextError(cfg.Context); err != nil {
			return nil, err
		}
		unit := &synthesisPlan.Units[unitIndex]
		timing := timings[unitIndex]
		if unit.Silent {
			continue
		}
		mono, err := cache.LoadMono(unit.Source)
		if err != nil {
			return nil, fmt.Errorf("read unit %q (%s): %w", unit.Alias, unit.Source, err)
		}
		if sampleRate == 0 {
			sampleRate = mono.SampleRate
		}
		if mono.SampleRate != sampleRate {
			mono, err = cache.LoadNormalized(unit.Source, sampleRate)
			if err != nil {
				return nil, fmt.Errorf("normalize unit %q: %w", unit.Alias, err)
			}
		}
		trimmed, err := audio.TrimPCM(mono, unit.OffsetMS, unit.CutoffMS)
		if err != nil {
			return nil, fmt.Errorf("trim unit %q: %w", unit.Alias, err)
		}

		targetMS := math.Max(1, timing.PreutteranceMS+unit.DurationMS+cfg.ReleaseMS)
		targetFrames := msToFrames(targetMS, sampleRate)
		sourceConsonantFrames := msToFrames(unit.ConsonantMS, sampleRate)
		sourcePreutteranceFrames := msToFrames(unit.PreutteranceMS, sampleRate)
		effectiveConsonantFrames := msToFrames(timing.ConsonantMS, sampleRate)
		wave := pcmFloats(trimmed.Data)
		sourceFrames := len(wave)
		appliedPitch := 1.0
		if cfg.ApplyPitch {
			appliedPitch = unit.PitchFactor * intonation[unitIndex]
		}
		if cfg.PitchCurve != nil {
			positionMS := unit.NoteStartMS - timing.PreutteranceMS
			spanMS := float64(len(wave)) / float64(sampleRate) * 1000
			wave = resampleForPitchCurve(wave, appliedPitch, cfg.PitchCurve, positionMS, spanMS)
		} else {
			wave = resampleForPitch(wave, appliedPitch)
		}
		if appliedPitch > 0 {
			consonantFactor := clampPitchFactor(appliedPitch)
			if cfg.PitchCurve != nil {
				consonantFactor = clampPitchFactor(appliedPitch * pitchCurveFactorAt(cfg.PitchCurve, unit.NoteStartMS-timing.PreutteranceMS))
			}
			sourceConsonantFrames = int(math.Round(float64(sourceConsonantFrames) / consonantFactor))
			sourcePreutteranceFrames = int(math.Round(float64(sourcePreutteranceFrames) / consonantFactor))
		}
		speechSourceConsonantFrames := sourceConsonantFrames
		if synthesisPlan.SpeechTiming && cfg.PitchCurve != nil {
			positionMS := unit.NoteStartMS - timing.PreutteranceMS
			spanMS := framesToMS(sourceFrames, sampleRate)
			sourcePreutteranceFrames = speechPitchAnchor(sourceFrames, msToFrames(unit.PreutteranceMS, sampleRate), appliedPitch, cfg.PitchCurve, positionMS, spanMS)
			speechSourceConsonantFrames = speechPitchAnchor(sourceFrames, msToFrames(unit.ConsonantMS, sampleRate), appliedPitch, cfg.PitchCurve, positionMS, spanMS)
		}
		prepared = append(prepared, preparedWaveformUnit{unitIndex: unitIndex, timing: timing, wave: wave,
			targetFrames: targetFrames, sourceConsonantFrames: sourceConsonantFrames,
			sourcePreutteranceFrames:    sourcePreutteranceFrames,
			speechSourceConsonantFrames: speechSourceConsonantFrames,
			effectiveConsonantFrames:    effectiveConsonantFrames})
	}
	retimeUnit := func(index int) error {
		if err := contextError(cfg.Context); err != nil {
			return err
		}
		item := &prepared[index]
		unit := &synthesisPlan.Units[item.unitIndex]
		var wave []float64
		var err error
		if synthesisPlan.SpeechTiming && unit.Role == "mora" && unit.SpeechProfile != nil && unit.SpeechProfile.Applied {
			var targetFixed int
			wave, targetFixed, unit.SpeechRetimeApplied = speechRetime(item.wave, item.targetFrames, item.sourcePreutteranceFrames,
				item.speechSourceConsonantFrames, msToFrames(item.timing.PreutteranceMS, sampleRate), item.effectiveConsonantFrames, sampleRate, speechStop(synthesisPlan, *unit))
			if unit.SpeechRetimeApplied {
				item.timing.ConsonantMS = framesToMS(targetFixed, sampleRate)
				timings[item.unitIndex].ConsonantMS = item.timing.ConsonantMS
				unit.EffectiveConsonantMS = item.timing.ConsonantMS
			}
		}
		if !unit.SpeechRetimeApplied {
			wave, err = retime(item.wave, item.targetFrames, item.sourceConsonantFrames, item.effectiveConsonantFrames, sampleRate)
		}
		if err != nil {
			return err
		}
		energy := synthesisPlan.Units[item.unitIndex].EnergyFactor
		if energy > 0 {
			for frame := range wave {
				wave[frame] *= energy
			}
		}
		if unit.ResamplerVolumeOverride && unit.ResamplerVolume >= 0 {
			gain := float64(unit.ResamplerVolume) / 100.0
			for frame := range wave {
				wave[frame] *= gain
			}
		}
		item.wave = wave
		return nil
	}
	retimeErrors := make([]error, len(prepared))
	if parallelRetime {
		workerCount := min(len(prepared), maxParallelRetimeUnits)
		jobs := make(chan int)
		var workers sync.WaitGroup
		workers.Add(workerCount)
		for range workerCount {
			go func() {
				defer workers.Done()
				for index := range jobs {
					retimeErrors[index] = retimeUnit(index)
				}
			}()
		}
		for index := range prepared {
			jobs <- index
		}
		close(jobs)
		workers.Wait()
	} else {
		for index := range prepared {
			retimeErrors[index] = retimeUnit(index)
		}
	}
	for index, err := range retimeErrors {
		if err != nil {
			return nil, fmt.Errorf("retime unit %q: %w", synthesisPlan.Units[prepared[index].unitIndex].Alias, err)
		}
	}

	leadingFrames := msToFrames(leadingMS, sampleRate)
	for _, item := range prepared {
		if err := contextError(cfg.Context); err != nil {
			return nil, err
		}
		unitIndex := item.unitIndex
		unit := &synthesisPlan.Units[unitIndex]
		timing := item.timing
		if unitIndex < len(timings) {
			// speechRetime が更新した子音境界を境界補正にも引き継ぐ
			timing = timings[unitIndex]
		}
		wave := item.wave

		startFrame := msToFramesSigned(unit.NoteStartMS-timing.PreutteranceMS, sampleRate) + leadingFrames
		sourceStart := 0
		if startFrame < 0 {
			sourceStart = -startFrame
			startFrame = 0
		}
		if sourceStart >= len(wave) {
			continue
		}
		rendered = append(rendered, renderedUnit{
			Index: unitIndex, Unit: *unit, Timing: timing, Wave: wave,
			StartFrame:   startFrame,
			FadeInFrames: msToFrames(fadeInDurationMS(timing), sampleRate),
		})
		endFrame := startFrame + len(wave) - sourceStart
		if endFrame > len(mix) {
			mix = append(mix, make([]float64, endFrame-len(mix))...)
			mixWeights = append(mixWeights, make([]float64, endFrame-len(mixWeights))...)
		}

		fadeInMS := fadeInDurationMS(timing)
		fadeInFrames := msToFrames(fadeInMS, sampleRate)
		fadeOutFrames := msToFrames(cfg.ReleaseMS, sampleRate)
		for sourceFrame := sourceStart; sourceFrame < len(wave); sourceFrame++ {
			if sourceFrame%4096 == 0 {
				if err := contextError(cfg.Context); err != nil {
					return nil, err
				}
			}
			gain := envelope(sourceFrame, len(wave), fadeInFrames, fadeOutFrames)
			position := startFrame + sourceFrame - sourceStart
			gain *= handoffGain(position-leadingFrames, unitIndex, synthesisPlan, timings, sampleRate)
			mix[position] += wave[sourceFrame] * gain
			mixWeights[position] += gain
		}
	}
	applyBoundaryBridges(mix, mixWeights, rendered, synthesisPlan, cfg, sampleRate)
	if sampleRate == 0 || len(mix) == 0 {
		return nil, errors.New("render produced no samples")
	}

	minimumFrames := msToFrames(synthesisPlan.DurationMS+cfg.ReleaseMS, sampleRate) + leadingFrames
	if len(mix) < minimumFrames {
		padding := minimumFrames - len(mix)
		mix = append(mix, make([]float64, padding)...)
		mixWeights = append(mixWeights, make([]float64, padding)...)
	}
	for i := range mix {
		if mixWeights[i] > 1 {
			mix[i] /= mixWeights[i]
		}
	}
	preventClipping(mix, 0.98)
	return &audio.PCM{SampleRate: sampleRate, Channels: 1, Data: floatPCM(mix)}, nil
}

func leadingPreutteranceMS(units []plan.Unit, timings []effectiveTiming) float64 {
	leading := 0.0
	for index, unit := range units {
		if unit.Silent || index >= len(timings) {
			continue
		}
		start := unit.NoteStartMS - timings[index].PreutteranceMS
		if start < 0 {
			leading = max(leading, -start)
		}
	}
	return leading
}

func handoffGain(globalFrame, unitIndex int, synthesisPlan *plan.Plan, timings []effectiveTiming, sampleRate int) float64 {
	if unitIndex+1 >= len(synthesisPlan.Units) {
		return 1
	}
	unit := synthesisPlan.Units[unitIndex]
	next := synthesisPlan.Units[unitIndex+1]
	if !unitsShareHandoff(unit, next) {
		return 1
	}
	nextTiming := timings[unitIndex+1]
	start := msToFramesSigned(next.NoteStartMS-nextTiming.PreutteranceMS, sampleRate)
	end := start + msToFrames(fadeInDurationMS(nextTiming), sampleRate)
	if globalFrame <= start {
		return 1
	}
	if globalFrame >= end || end <= start {
		return 0
	}
	progress := float64(globalFrame-start) / float64(end-start)
	return 1 - smoothstep(progress)
}

func unitsShareHandoff(previous, next plan.Unit) bool {
	previousRole := previous.Role
	if previousRole == "" {
		previousRole = "mora"
	}
	nextRole := next.Role
	if nextRole == "" {
		nextRole = "mora"
	}
	if nextRole == "transition" {
		return previousRole == "mora" && next.Position == previous.Position+1
	}
	if previousRole == "transition" {
		return nextRole == "mora" && next.Position == previous.Position
	}
	return next.Position == previous.Position+1
}

func envelope(frame, total, fadeIn, fadeOut int) float64 {
	gain := 1.0
	if fadeIn > 0 && frame < fadeIn {
		gain = smoothstep(float64(frame) / float64(fadeIn))
	}
	remaining := total - 1 - frame
	if fadeOut > 0 && remaining < fadeOut {
		outGain := smoothstep(float64(remaining) / float64(fadeOut))
		if outGain < gain {
			gain = outGain
		}
	}
	if gain < 0 {
		return 0
	}
	return gain
}

func preventClipping(data []float64, limit float64) {
	peak := 0.0
	for _, value := range data {
		if absolute := math.Abs(value); absolute > peak {
			peak = absolute
		}
	}
	if peak <= limit || peak == 0 {
		return
	}
	scale := limit / peak
	for i := range data {
		data[i] *= scale
	}
}
