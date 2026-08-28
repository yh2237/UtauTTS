package render

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
	"utautts/internal/pitch"
	"utautts/internal/plan"
)

const worldlineFrameMS = 10.0

type worldlineManifest struct {
	Engine        string                  `json:"engine,omitempty"`
	WorldlinePath string                  `json:"worldline_path"`
	GPUPath       string                  `json:"gpu_path,omitempty"`
	OutputPath    string                  `json:"output_path"`
	SampleRate    int                     `json:"sample_rate"`
	F0Curve       []float64               `json:"f0_curve"`
	Units         []worldlineManifestUnit `json:"units"`
}

func renderOpenUtauClassicWorldlineFaithful(synthesisPlan *plan.Plan, cfg Config) (*audio.PCM, error) {
	return renderWorldlineEngine(synthesisPlan, cfg, "classic-worldline-faithful", true)
}

func renderOpenUtauClassicWorldlineFaithfulGPU(synthesisPlan *plan.Plan, cfg Config) (*audio.PCM, error) {
	return renderWorldlineEngine(synthesisPlan, cfg, "classic-worldline-faithful-gpu", true)
}

func renderOpenUtauWorldlineRFaithful(synthesisPlan *plan.Plan, cfg Config) (*audio.PCM, error) {
	return renderWorldlineEngine(synthesisPlan, cfg, "worldline-r-faithful", false)
}

type worldlineManifestUnit struct {
	CacheKey          string                   `json:"cache_key,omitempty"`
	Source            string                   `json:"source"`
	FRQPath           string                   `json:"frq_path,omitempty"`
	PositionMS        float64                  `json:"position_ms"`
	SkipMS            float64                  `json:"skip_ms"`
	LengthMS          float64                  `json:"length_ms"`
	FadeInMS          float64                  `json:"fade_in_ms"`
	FadeOutMS         float64                  `json:"fade_out_ms"`
	OffsetMS          float64                  `json:"offset_ms"`
	RequiredLengthMS  float64                  `json:"required_length_ms"`
	ConsonantMS       float64                  `json:"consonant_ms"`
	CutoffMS          float64                  `json:"cutoff_ms"`
	Tone              int                      `json:"tone"`
	ConsonantVelocity float64                  `json:"consonant_velocity"`
	PitchStartMS      float64                  `json:"pitch_start_ms,omitempty"`
	PitchLengthMS     float64                  `json:"pitch_length_ms,omitempty"`
	Volume            float64                  `json:"volume,omitempty"`
	Modulation        float64                  `json:"modulation,omitempty"`
	Tempo             float64                  `json:"tempo,omitempty"`
	Envelope          []worldlineEnvelopePoint `json:"envelope,omitempty"`
}

type worldlineEnvelopePoint struct {
	XMS float64 `json:"x_ms"`
	Y   float64 `json:"y"`
}

func renderWorldlineEngine(synthesisPlan *plan.Plan, cfg Config, engine string, localSourcePitch bool) (*audio.PCM, error) {
	if synthesisPlan == nil || len(synthesisPlan.Units) == 0 {
		return nil, errors.New("empty synthesis plan")
	}
	if cfg.CVVCTiming == "" {
		cfg.CVVCTiming = CVVCTimingLegacy
	}
	if cfg.CVVCTiming != CVVCTimingLegacy && cfg.CVVCTiming != CVVCTimingSequential {
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
	library, err := resolveWorldlineLibrary(cfg.WorldlinePath)
	if err != nil {
		return nil, err
	}
	bridge, err := resolveWorldlineBridge(cfg.WorldlineBridgePath)
	if err != nil {
		return nil, err
	}
	gpuPath := ""
	if strings.HasSuffix(engine, "-gpu") {
		if err := gpuWaveformAvailable(); err != nil {
			return nil, err
		}
		gpuPath, err = gpuWaveformLibraryPath()
		if err != nil {
			return nil, err
		}
	}

	cache := newSourceCache()
	timings := make([]effectiveTiming, len(synthesisPlan.Units))
	var classicTimings []openUtauClassicTiming
	phraseStartMS := 0.0
	faithfulClassic := strings.HasPrefix(engine, "classic-worldline-faithful")
	faithfulWorldlineR := engine == "worldline-r-faithful"
	openUtauTiming := faithfulClassic || faithfulWorldlineR
	if openUtauTiming {
		classicTimings, phraseStartMS = openUtauClassicTimings(synthesisPlan.Units, cfg.CVVCTiming)
	}
	leadingMS := limitLeadingPreutterance(math.Max(0, -phraseStartMS), cfg.LeadingPreutteranceMS)
	synthesisPlan.LeadingMarginMS = leadingMS
	for i := range synthesisPlan.Units {
		unit := &synthesisPlan.Units[i]
		timings[i] = normalizeTiming(*unit, cfg.ReleaseMS)
		if len(classicTimings) == len(synthesisPlan.Units) && !unit.Silent {
			timings[i].preutteranceMS = classicTimings[i].preutter
			timings[i].overlapMS = classicTimings[i].overlap
			timings[i].consonantMS = unit.ConsonantMS
			timings[i].scale = 1
		}
		unit.TimingScale = timings[i].scale
		unit.EffectivePreutteranceMS = timings[i].preutteranceMS
		unit.EffectiveConsonantMS = timings[i].consonantMS
		unit.EffectiveOverlapMS = timings[i].overlapMS
		unit.IntonationFactor = 1
	}
	intonation := identityFactors(len(synthesisPlan.Units))
	if cfg.ApplyPitch {
		intonation = analyzeIntonation(synthesisPlan, timings, &cache, cfg.IntonationStrength)
	}
	pitches, sampleRate, err := measureWorldlinePitches(synthesisPlan, &cache)
	if err != nil {
		return nil, err
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
	frameMS := worldlineFrameMS
	curveStartMS := 0.0
	curveDurationMS := synthesisPlan.DurationMS + cfg.ReleaseMS
	if faithfulWorldlineR {
		curveStartMS = -leadingMS
		curveDurationMS += leadingMS
	}
	f0Curve := worldlineF0CurveAtOffset(synthesisPlan, pitches, pitchFactors, reference,
		max(2, int(math.Ceil(curveDurationMS/frameMS))+2), frameMS, curveStartMS)
	if localSourcePitch {
		f0Curve = worldlineLocalF0Curve(synthesisPlan, pitches, pitchFactors, reference, len(f0Curve))
	}
	manifest := worldlineManifest{
		Engine:        engine,
		WorldlinePath: library,
		GPUPath:       gpuPath,
		SampleRate:    sampleRate,
		F0Curve:       f0Curve,
	}
	for frame := range manifest.F0Curve {
		manifest.F0Curve[frame] *= pitchCurveFactorAt(cfg.PitchCurve, curveStartMS+float64(frame)*frameMS)
	}
	tempDir, err := os.MkdirTemp("", "utautts-worldline-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	// 全ユニットをフレーズのレートへ統一してからbridgeへ渡す。
	// 再サンプル後は位置が合わないFRQを捨て、波形からF0を再測定する。
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
		volume, modulation, tempo := 100.0, 0.0, 120.0
		if unit.Role == "transition" {
			volume *= cfg.CVVCTransitionGain
		}
		var envelopePoints []worldlineEnvelopePoint
		pitchLengthMS := 0.0
		if strings.HasPrefix(engine, "classic-worldline-") || faithfulWorldlineR {
			// OpenUtau互換でbendを元のpreutteranceから始め、余分な先頭を後で飛ばす。
			pitchLeadingMS := unit.PreutteranceMS
			skipMS = math.Max(0, pitchLeadingMS-timing.preutteranceMS)
			pitchStartMS = unit.NoteStartMS - pitchLeadingMS
			durCorrection := 0.0
			if openUtauTiming {
				phoneTiming := classicTimings[i]
				// 呼び出し側に依存せず、クラシック方式のskipMSを非負に保つ。
				skipMS = math.Max(0, pitchLeadingMS-phoneTiming.preutter)
				durCorrection = phoneTiming.preutter - phoneTiming.tailIntrude + phoneTiming.tailOverlap
				envelopePoints = openUtauEnvelopeFromTiming(*unit, phoneTiming)
				if cfg.CVVCPreBoundaryFade && unit.Role == "transition" {
					envelopePoints = cvvcPreBoundaryEnvelope(envelopePoints, phoneTiming)
				}
				pitchLengthMS = envelopePoints[4].XMS + pitchLeadingMS
				positionMS = unit.NoteStartMS - phoneTiming.preutter + leadingMS
			}
			requiredLength = math.Max(unit.DurationMS+durCorrection+skipMS, unit.ConsonantMS)
			requiredLength = math.Ceil(requiredLength/50+0.5) * 50
			if cfg.WorldlineExactLength {
				requiredLength = unit.DurationMS
			}
			lengthMS = timing.preutteranceMS + unit.DurationMS + cfg.ReleaseMS
			consonantVelocity = 100
		}
		if !faithfulClassic && positionMS < 0 {
			leadingTrimMS := -positionMS
			skipMS += leadingTrimMS
			lengthMS -= leadingTrimMS
			positionMS = 0
		}
		source, frqPath := unit.Source, findFRQPath(unit.Source)
		if normalized, ok := normalizedSources[unit.Source]; ok {
			source = normalized
			frqPath = ""
		}
		fadeInMS := math.Max(2, timing.preutteranceMS-timing.overlapMS)
		fadeOutMS := cfg.ReleaseMS
		if faithfulWorldlineR && len(envelopePoints) == 5 {
			lengthMS = envelopePoints[4].XMS - envelopePoints[0].XMS
			fadeInMS = envelopePoints[1].XMS - envelopePoints[0].XMS
			fadeOutMS = envelopePoints[4].XMS - envelopePoints[3].XMS
		}
		manifest.Units = append(manifest.Units, worldlineManifestUnit{
			CacheKey: worldlineAnalysisCacheKey(source, frqPath, *unit, volume),
			Source:   source, FRQPath: frqPath, PositionMS: positionMS, SkipMS: skipMS,
			LengthMS: lengthMS, FadeInMS: fadeInMS,
			FadeOutMS: fadeOutMS, OffsetMS: unit.OffsetMS, RequiredLengthMS: requiredLength,
			ConsonantMS: unit.ConsonantMS, CutoffMS: unit.CutoffMS,
			Tone: int(math.Round(69 + 12*math.Log2(unitPitch/440))), ConsonantVelocity: consonantVelocity,
			PitchStartMS: pitchStartMS, Volume: volume, Modulation: modulation, Tempo: tempo,
			PitchLengthMS: pitchLengthMS, Envelope: envelopePoints,
		})
	}

	manifest.OutputPath = filepath.Join(tempDir, "output.wav")
	manifestPath := filepath.Join(tempDir, "manifest.json")
	data, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(manifestPath, data, 0o600); err != nil {
		return nil, err
	}
	ctx := cfg.Context
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if commandErr := invokeWorldlineBridge(ctx, bridge, manifestPath); commandErr != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("worldline bridge canceled: %w", ctxErr)
		}
		return nil, fmt.Errorf("worldline bridge failed: %w", commandErr)
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

type openUtauClassicTiming struct {
	preutter    float64
	overlap     float64
	tailIntrude float64
	tailOverlap float64
	overlapped  bool
}

func openUtauClassicTimings(units []plan.Unit, cvvcTiming string) ([]openUtauClassicTiming, float64) {
	result := make([]openUtauClassicTiming, len(units))
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
		}
		autoPreutter = math.Max(0, autoPreutter)
		result[index].preutter = autoPreutter
		result[index].overlap = autoOverlap
		result[index].overlapped = previous >= 0 && adjacent && autoOverlap > 0
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

func openUtauEnvelopeFromTiming(unit plan.Unit, timing openUtauClassicTiming) []worldlineEnvelopePoint {
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

func cvvcPreBoundaryEnvelope(points []worldlineEnvelopePoint, timing openUtauClassicTiming) []worldlineEnvelopePoint {
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

func resolveWorldlineLibrary(configured string) (string, error) {
	if configured == "" {
		return "", errors.New("worldline library is not configured by the renderer plugin")
	}
	if _, err := os.Stat(configured); err != nil {
		return "", fmt.Errorf("worldline library %q: %w", configured, err)
	}
	return configured, nil
}

func measureWorldlinePitches(synthesisPlan *plan.Plan, cache *sourceCache) ([]float64, int, error) {
	values := make([]float64, len(synthesisPlan.Units))
	sampleRate := 0
	for i, unit := range synthesisPlan.Units {
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
		trimmed, err := audio.TrimPCM(mono, unit.OffsetMS, unit.CutoffMS)
		if err != nil {
			return nil, 0, err
		}
		wave := pcmFloats(trimmed.Data)
		start := min(len(wave), msToFrames(unit.ConsonantMS, mono.SampleRate))
		end := min(len(wave), start+msToFrames(180, mono.SampleRate))
		if end-start >= msToFrames(30, mono.SampleRate) {
			values[i] = pitch.EstimateMedian(wave[start:end], mono.SampleRate)
		}
	}
	return stabilizeWorldlinePitches(values), sampleRate, nil
}

// stabilizeWorldlinePitchesは短い有声録音の倍音誤検出を補正する。
// 相互補正を避けるため低周波側を基準に高周波側だけを折り畳む。
func stabilizeWorldlinePitches(values []float64) []float64 {
	result := append([]float64(nil), values...)
	for index, value := range values {
		if value <= 0 {
			continue
		}
		neighbor := nearestWorldlinePitch(values, index)
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

func worldlineLocalF0Curve(synthesisPlan *plan.Plan, pitches, factors []float64, reference float64, length int) []float64 {
	targets := make([]float64, len(pitches))
	for index, value := range pitches {
		if value <= 0 {
			value = reference
		}
		targets[index] = value * factors[index]
	}
	curve := make([]float64, length)
	unitIndex := 0
	for frame := range curve {
		timeMS := float64(frame) * worldlineFrameMS
		for unitIndex+1 < len(synthesisPlan.Units) && synthesisPlan.Units[unitIndex+1].NoteStartMS <= timeMS {
			unitIndex++
		}
		curve[frame] = targets[unitIndex]
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
