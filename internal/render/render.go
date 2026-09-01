package render

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"sync"

	"utautts/internal/audio"
	"utautts/internal/pitch"
	"utautts/internal/plan"
)

type Config struct {
	Context                 context.Context
	ReleaseMS               float64
	ReleaseSet              bool
	LeadingPreutteranceMS   float64
	IntonationStrength      float64
	ApplyPitch              bool
	Backend                 string
	WorldlinePath           string
	WorldlineBridgePath     string
	WorldEnginePath         string
	ExternalResamplerPath   string
	WorldlineExactLength    bool
	BoundaryBridgeMS        float64
	BoundaryBridgeThreshold float64
	CVVCTiming              string
	CVVCTransitionGain      float64
	CVVCPreBoundaryFade     bool
	PitchCurve              *PitchCurve
}

const (
	CVVCTimingLegacy     = "legacy"
	CVVCTimingSequential = "sequential"
)

// MaxIntonationStrengthはユーザー向けイントネーション制御の上限値。
const MaxIntonationStrength = 4.0

// defaultReleaseMSは未指定時のリリース長。明示的な0にはReleaseSetを使う。
const defaultReleaseMS = 20.0

// rendererImplementationsは実行可能なbackendの一覧。表示情報はplugin.jsonに置く。
var rendererImplementations = map[string]func(*plan.Plan, Config) (*audio.PCM, error){
	"waveform":                      renderWaveform,
	"openutau-worldline-r-faithful": renderOpenUtauWorldlineRFaithful,
	"utautts-world-phrase":          renderUtauTTSWorldPhrase,
	"utau-external-resampler":       renderUtauExternalResampler,
}

func IsKnownRenderer(id string) bool {
	if id == "" {
		return true
	}
	_, ok := rendererImplementations[id]
	return ok
}

var boundaryBridgeRenderers = map[string]struct{}{
	"": {}, "waveform": {},
}

type PitchCurve struct {
	FrameMS float64   `json:"frame_ms"`
	Cents   []float64 `json:"cents"`
}

type sourceCache struct {
	raw        map[string]*audio.PCM
	mono       map[string]*audio.PCM
	normalized map[sourceCacheKey]*audio.PCM
}

type sourceCacheKey struct {
	path       string
	sampleRate int
}

func newSourceCache() sourceCache {
	return sourceCache{
		raw:        make(map[string]*audio.PCM),
		mono:       make(map[string]*audio.PCM),
		normalized: make(map[sourceCacheKey]*audio.PCM),
	}
}

// 音源録音は候補探索とレンダリングで再利用されるため、デコード結果を保持する。
const maxWAVCacheBytes = 256 << 20 // デコード済み音源 256 MiB

type wavCacheEntry struct {
	path    string
	size    int64
	modTime int64
	pcm     *audio.PCM
}

type wavCache struct {
	mu     sync.Mutex
	byPath map[string]*list.Element
	order  *list.List
	bytes  int64
}

var globalWAVCache = wavCache{
	byPath: make(map[string]*list.Element),
	order:  list.New(),
}

const maxUnitPitchCacheEntries = 4096

type unitPitchCacheKey struct {
	path                      string
	size, modTime             int64
	offset, cutoff, consonant uint64
}

type unitPitchCacheEntry struct {
	key   unitPitchCacheKey
	value float64
}

var globalUnitPitchCache = struct {
	sync.Mutex
	entries map[unitPitchCacheKey]*list.Element
	order   *list.List
}{entries: make(map[unitPitchCacheKey]*list.Element), order: list.New()}

func (c *wavCache) remove(element *list.Element) {
	entry := element.Value.(*wavCacheEntry)
	c.bytes -= int64(len(entry.pcm.Data)) * 2
	delete(c.byPath, entry.path)
	c.order.Remove(element)
}

func (c *wavCache) evict() {
	for c.bytes > maxWAVCacheBytes && c.order.Len() > 0 {
		c.remove(c.order.Back())
	}
}

// loadWAVCachedはサイズと更新時刻で変更を検知し、古いWAVから追い出す。
func loadWAVCached(path string) (*audio.PCM, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	modTime := info.ModTime().UnixNano()
	globalWAVCache.mu.Lock()
	defer globalWAVCache.mu.Unlock()
	if element, ok := globalWAVCache.byPath[path]; ok {
		entry := element.Value.(*wavCacheEntry)
		if entry.size == info.Size() && entry.modTime == modTime {
			globalWAVCache.order.MoveToFront(element)
			return entry.pcm, nil
		}
		globalWAVCache.remove(element)
	}
	pcm, err := audio.ReadWav(path)
	if err != nil {
		return nil, err
	}
	entry := &wavCacheEntry{path: path, size: info.Size(), modTime: modTime, pcm: pcm}
	element := globalWAVCache.order.PushFront(entry)
	globalWAVCache.byPath[path] = element
	globalWAVCache.bytes += int64(len(pcm.Data)) * 2
	globalWAVCache.evict()
	return pcm, nil
}

// ClearWAVCacheは音源更新後にキャッシュ済み録音を破棄する。
func ClearWAVCache() {
	globalWAVCache.mu.Lock()
	defer globalWAVCache.mu.Unlock()
	for element := globalWAVCache.order.Front(); element != nil; {
		next := element.Next()
		globalWAVCache.remove(element)
		element = next
	}
	globalUnitPitchCache.Lock()
	globalUnitPitchCache.entries = make(map[unitPitchCacheKey]*list.Element)
	globalUnitPitchCache.order.Init()
	globalUnitPitchCache.Unlock()
}

func estimateUnitPitch(unit plan.Unit, mono *audio.PCM) (float64, error) {
	key := unitPitchCacheKey{
		path: unit.Source, offset: math.Float64bits(unit.OffsetMS), cutoff: math.Float64bits(unit.CutoffMS),
		consonant: math.Float64bits(unit.ConsonantMS),
	}
	if info, err := os.Stat(unit.Source); err == nil {
		key.size = info.Size()
		key.modTime = info.ModTime().UnixNano()
	}
	globalUnitPitchCache.Lock()
	if element, found := globalUnitPitchCache.entries[key]; found {
		globalUnitPitchCache.order.MoveToFront(element)
		value := element.Value.(unitPitchCacheEntry).value
		globalUnitPitchCache.Unlock()
		return value, nil
	}
	globalUnitPitchCache.Unlock()

	trimmed, err := audio.TrimPCM(mono, unit.OffsetMS, unit.CutoffMS)
	if err != nil {
		return 0, err
	}
	wave := pcmFloats(trimmed.Data)
	start := min(len(wave), msToFrames(unit.ConsonantMS, mono.SampleRate))
	end := min(len(wave), start+msToFrames(180, mono.SampleRate))
	value := 0.0
	if end-start >= msToFrames(30, mono.SampleRate) {
		value = pitch.EstimateMedian(wave[start:end], mono.SampleRate)
	}

	globalUnitPitchCache.Lock()
	if element, found := globalUnitPitchCache.entries[key]; found {
		globalUnitPitchCache.order.MoveToFront(element)
		value = element.Value.(unitPitchCacheEntry).value
	} else {
		element := globalUnitPitchCache.order.PushFront(unitPitchCacheEntry{key: key, value: value})
		globalUnitPitchCache.entries[key] = element
		if globalUnitPitchCache.order.Len() > maxUnitPitchCacheEntries {
			oldest := globalUnitPitchCache.order.Back()
			delete(globalUnitPitchCache.entries, oldest.Value.(unitPitchCacheEntry).key)
			globalUnitPitchCache.order.Remove(oldest)
		}
	}
	globalUnitPitchCache.Unlock()
	return value, nil
}

type effectiveTiming struct {
	preutteranceMS float64
	consonantMS    float64
	overlapMS      float64
	scale          float64
}

func Render(synthesisPlan *plan.Plan, cfg Config) (*audio.PCM, error) {
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
		cfg.ReleaseMS = defaultReleaseMS
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
	if backend, ok := rendererImplementations[cfg.Backend]; ok {
		return backend(synthesisPlan, cfg)
	}
	return nil, fmt.Errorf("unknown renderer backend %q", cfg.Backend)
}

func effectiveUnitPitchFactor(unit plan.Unit, applyPitch bool) float64 {
	if !applyPitch || unit.PitchFactor <= 0 {
		return 1
	}
	return unit.PitchFactor
}

func rendererSupportsBoundaryBridge(renderer string) bool {
	_, ok := boundaryBridgeRenderers[renderer]
	return ok
}

func pitchCurveFactorAt(curve *PitchCurve, timeMS float64) float64 {
	if curve == nil || curve.FrameMS <= 0 || len(curve.Cents) == 0 {
		return 1
	}
	position := math.Max(0, timeMS) / curve.FrameMS
	left := int(math.Floor(position))
	if left >= len(curve.Cents)-1 {
		return math.Pow(2, curve.Cents[len(curve.Cents)-1]/1200)
	}
	progress := position - float64(left)
	cents := curve.Cents[left]*(1-progress) + curve.Cents[left+1]*progress
	return math.Pow(2, cents/1200)
}

func renderWaveform(synthesisPlan *plan.Plan, cfg Config) (*audio.PCM, error) {
	return renderWaveformWithStretch(synthesisPlan, cfg, false, func(source []float64, targetFrames, sourcePrefixFrames, targetPrefixFrames, sampleRate int) ([]float64, error) {
		return retimeWithCompressedPrefixUsing(source, targetFrames, sourcePrefixFrames, targetPrefixFrames, sampleRate, wsolaStretch)
	})
}

type preparedWaveformUnit struct {
	unitIndex                int
	timing                   effectiveTiming
	wave                     []float64
	targetFrames             int
	sourceConsonantFrames    int
	effectiveConsonantFrames int
}

// CUDA資源の過剰生成を防ぎつつGPUを活用できる数に制限する。
const maxParallelGPUUnits = 32

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
		timings[i] = normalizeTiming(*unit, cfg.ReleaseMS)
		unit.TimingScale = timings[i].scale
		unit.EffectivePreutteranceMS = timings[i].preutteranceMS
		unit.EffectiveConsonantMS = timings[i].consonantMS
		unit.EffectiveOverlapMS = timings[i].preutteranceMS - fadeInDurationMS(timings[i])
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
		mono, err := cache.loadMono(unit.Source)
		if err != nil {
			return nil, fmt.Errorf("read unit %q (%s): %w", unit.Alias, unit.Source, err)
		}
		if sampleRate == 0 {
			sampleRate = mono.SampleRate
		}
		if mono.SampleRate != sampleRate {
			mono, err = cache.loadNormalized(unit.Source, sampleRate)
			if err != nil {
				return nil, fmt.Errorf("normalize unit %q: %w", unit.Alias, err)
			}
		}
		trimmed, err := audio.TrimPCM(mono, unit.OffsetMS, unit.CutoffMS)
		if err != nil {
			return nil, fmt.Errorf("trim unit %q: %w", unit.Alias, err)
		}

		targetMS := math.Max(1, timing.preutteranceMS+unit.DurationMS+cfg.ReleaseMS)
		targetFrames := msToFrames(targetMS, sampleRate)
		sourceConsonantFrames := msToFrames(unit.ConsonantMS, sampleRate)
		effectiveConsonantFrames := msToFrames(timing.consonantMS, sampleRate)
		wave := pcmFloats(trimmed.Data)
		appliedPitch := 1.0
		if cfg.ApplyPitch {
			appliedPitch = unit.PitchFactor * intonation[unitIndex]
		}
		if cfg.PitchCurve != nil {
			positionMS := unit.NoteStartMS - timing.preutteranceMS
			spanMS := float64(len(wave)) / float64(sampleRate) * 1000
			wave = resampleForPitchCurve(wave, appliedPitch, cfg.PitchCurve, positionMS, spanMS)
		} else {
			wave = resampleForPitch(wave, appliedPitch)
		}
		if appliedPitch > 0 {
			consonantFactor := clampPitchFactor(appliedPitch)
			if cfg.PitchCurve != nil {
				consonantFactor = clampPitchFactor(appliedPitch * pitchCurveFactorAt(cfg.PitchCurve, unit.NoteStartMS-timing.preutteranceMS))
			}
			sourceConsonantFrames = int(math.Round(float64(sourceConsonantFrames) / consonantFactor))
		}
		prepared = append(prepared, preparedWaveformUnit{unitIndex: unitIndex, timing: timing, wave: wave,
			targetFrames: targetFrames, sourceConsonantFrames: sourceConsonantFrames,
			effectiveConsonantFrames: effectiveConsonantFrames})
	}
	retimeUnit := func(index int) error {
		if err := contextError(cfg.Context); err != nil {
			return err
		}
		item := &prepared[index]
		wave, err := retime(item.wave, item.targetFrames, item.sourceConsonantFrames, item.effectiveConsonantFrames, sampleRate)
		if err != nil {
			return err
		}
		energy := synthesisPlan.Units[item.unitIndex].EnergyFactor
		if energy > 0 {
			for frame := range wave {
				wave[frame] *= energy
			}
		}
		item.wave = wave
		return nil
	}
	retimeErrors := make([]error, len(prepared))
	if parallelRetime {
		workerCount := min(len(prepared), maxParallelGPUUnits)
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
		wave := item.wave

		startFrame := msToFramesSigned(unit.NoteStartMS-timing.preutteranceMS, sampleRate) + leadingFrames
		sourceStart := 0
		if startFrame < 0 {
			sourceStart = -startFrame
			startFrame = 0
		}
		if sourceStart >= len(wave) {
			continue
		}
		rendered = append(rendered, renderedUnit{
			index: unitIndex, unit: *unit, timing: timing, wave: wave,
			startFrame:   startFrame,
			fadeInFrames: msToFrames(fadeInDurationMS(timing), sampleRate),
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
		start := unit.NoteStartMS - timings[index].preutteranceMS
		if start < 0 {
			leading = max(leading, -start)
		}
	}
	return leading
}

// limitLeadingPreutteranceは0を自動扱いにし、指定時だけ文頭側の保持区間を制限する。
func limitLeadingPreutterance(required, maximum float64) float64 {
	if maximum <= 0 {
		return required
	}
	return math.Min(required, maximum)
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return fmt.Errorf("render canceled: %w", ctx.Err())
	default:
		return nil
	}
}

func identityFactors(size int) []float64 {
	result := make([]float64, size)
	for index := range result {
		result[index] = 1
	}
	return result
}

func analyzeIntonation(synthesisPlan *plan.Plan, timings []effectiveTiming, cache *sourceCache, strength float64) []float64 {
	pitches := make([]float64, len(synthesisPlan.Units))
	for i, unit := range synthesisPlan.Units {
		if unit.Silent || unit.Role == "transition" {
			continue
		}
		mono, err := cache.loadMono(unit.Source)
		if err != nil {
			continue
		}
		pitches[i], err = estimateUnitPitch(unit, mono)
		if err != nil {
			continue
		}
	}
	return analyzeIntonationFromPitches(synthesisPlan, timings, pitches, strength)
}

func analyzeIntonationFromPitches(synthesisPlan *plan.Plan, timings []effectiveTiming, pitches []float64, strength float64) []float64 {
	factors := identityFactors(len(synthesisPlan.Units))
	strength = math.Max(0, math.Min(MaxIntonationStrength, strength))
	if strength == 0 {
		return factors
	}
	pitches = stabilizeWorldlinePitches(pitches)
	voiced := nonzeroFloats(pitches)
	reference := medianFloat(voiced)
	if reference <= 0 {
		return factors
	}
	for i := range pitches {
		if pitches[i] <= 0 {
			continue
		}
		for pitches[i] > reference*1.6 {
			pitches[i] /= 2
		}
		for pitches[i] < reference/1.6 {
			pitches[i] *= 2
		}
	}

	for start := 0; start < len(synthesisPlan.Units); {
		if synthesisPlan.Units[start].Role == "transition" {
			start++
			continue
		}
		end := start + 1
		lastPosition := synthesisPlan.Units[start].Position
		for end < len(synthesisPlan.Units) {
			unit := synthesisPlan.Units[end]
			if unit.Role == "transition" {
				end++
				continue
			}
			if unit.Position != lastPosition+1 {
				break
			}
			lastPosition = unit.Position
			end++
		}
		moraCount := 0
		for index := start; index < end; index++ {
			if synthesisPlan.Units[index].Role != "transition" {
				moraCount++
			}
		}
		moraIndex := 0
		for i := start; i < end; i++ {
			if synthesisPlan.Units[i].Role == "transition" {
				continue
			}
			position := 0.0
			if moraCount > 1 {
				position = float64(moraIndex) / float64(moraCount-1)
			}
			semitones := 0.3 - 0.8*position
			if moraIndex == 0 {
				semitones -= 0.35
			}
			if moraIndex == 1 {
				semitones += 0.25
			}
			target := reference * math.Pow(2, semitones/12)
			unit := &synthesisPlan.Units[i]
			unit.SourceF0Hz = pitches[i]
			if pitches[i] > 0 {
				effectiveStrength := strength
				if timings[i].scale < 1 {
					effectiveStrength *= math.Max(0.25, timings[i].scale)
				}
				factor := math.Pow(target/pitches[i], effectiveStrength)
				maxShift := 0.08 * math.Max(1, strength)
				factors[i] = math.Max(1-maxShift, math.Min(1+maxShift, factor))
				pitchFactor := unit.PitchFactor
				if pitchFactor <= 0 {
					pitchFactor = 1
				}
				unit.TargetF0Hz = pitches[i] * factors[i] * pitchFactor
			}
			unit.IntonationFactor = factors[i]
			moraIndex++
		}
		start = end
	}
	return factors
}

func medianFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	values = append([]float64(nil), values...)
	sort.Float64s(values)
	middle := len(values) / 2
	if len(values)%2 == 1 {
		return values[middle]
	}
	return (values[middle-1] + values[middle]) / 2
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
	start := msToFramesSigned(next.NoteStartMS-nextTiming.preutteranceMS, sampleRate)
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

func fadeInDurationMS(timing effectiveTiming) float64 {
	// 特殊なoto設定でも前後のゲインが同時に0にならないよう重なりを確保する。
	return math.Max(6, timing.preutteranceMS-timing.overlapMS)
}

func normalizeTiming(unit plan.Unit, releaseMS float64) effectiveTiming {
	preutterance := math.Max(0, unit.PreutteranceMS)
	overlap := unit.OverlapMS
	consonant := math.Max(0, unit.ConsonantMS)
	scale := 1.0

	if preutterance > math.Max(120, unit.DurationMS*1.5) {
		effectivePreutterance := math.Max(80, unit.DurationMS*0.75)
		scale = effectivePreutterance / preutterance
		preutterance = effectivePreutterance
		if overlap > 0 {
			overlap *= scale
		}
		consonant *= scale
	}
	overlap = math.Min(overlap, preutterance)

	if scale < 1 {
		targetMS := preutterance + unit.DurationMS + releaseMS
		minimumTailMS := releaseMS + math.Max(40, unit.DurationMS*0.35)
		consonant = math.Min(consonant, math.Max(0, targetMS-minimumTailMS))
	}
	return effectiveTiming{preutterance, consonant, overlap, scale}
}

func resampleForPitch(source []float64, factor float64) []float64 {
	if len(source) < 16 || factor <= 0 || math.Abs(factor-1) < 0.001 {
		return append([]float64(nil), source...)
	}
	factor = clampPitchFactor(factor)
	return linearResample(source, max(16, int(math.Round(float64(len(source))/factor))))
}

// resampleForPitchCurveは1/f(t)を積分し、時間変化するピッチでsourceを伸縮する。
// 平坦なカーブはresampleForPitchと同じ結果になる。
func resampleForPitchCurve(source []float64, baseFactor float64, curve *PitchCurve, startMS, spanMS float64) []float64 {
	if len(source) < 16 || baseFactor <= 0 {
		return append([]float64(nil), source...)
	}
	if curve == nil || curve.FrameMS <= 0 || len(curve.Cents) == 0 {
		return resampleForPitch(source, baseFactor)
	}
	span := math.Max(1e-3, spanMS)
	cumulative := make([]float64, len(source)+1)
	for i := range source {
		frame := 0.0
		if len(source) > 1 {
			frame = float64(i) / float64(len(source)-1)
		}
		factor := clampPitchFactor(baseFactor * pitchCurveFactorAt(curve, startMS+span*frame))
		cumulative[i+1] = cumulative[i] + 1/factor
	}
	targetFrames := max(16, int(math.Round(cumulative[len(source)])))
	result := make([]float64, targetFrames)
	segment := 0
	for output := range result {
		for segment < len(source)-1 && float64(output) >= cumulative[segment+1] {
			segment++
		}
		fraction := 0.0
		if segment < len(source)-1 {
			width := cumulative[segment+1] - cumulative[segment]
			if width > 0 {
				fraction = (float64(output) - cumulative[segment]) / width
			}
		}
		right := source[segment]
		if segment+1 < len(source) {
			right = source[segment+1]
		}
		result[output] = source[segment] + (right-source[segment])*fraction
	}
	return result
}

func clampPitchFactor(factor float64) float64 {
	return math.Max(0.75, math.Min(1.35, factor))
}

func (c *sourceCache) ensureMaps() {
	if c.raw == nil {
		c.raw = make(map[string]*audio.PCM)
	}
	if c.mono == nil {
		c.mono = make(map[string]*audio.PCM)
	}
	if c.normalized == nil {
		c.normalized = make(map[sourceCacheKey]*audio.PCM)
	}
}

func (c *sourceCache) load(path string) (*audio.PCM, error) {
	c.ensureMaps()
	if pcm, ok := c.raw[path]; ok {
		return pcm, nil
	}
	pcm, err := loadWAVCached(path)
	if err != nil {
		return nil, err
	}
	c.raw[path] = pcm
	return pcm, nil
}

func (c *sourceCache) loadMono(path string) (*audio.PCM, error) {
	c.ensureMaps()
	if pcm, ok := c.mono[path]; ok {
		return pcm, nil
	}
	raw, err := c.load(path)
	if err != nil {
		return nil, err
	}
	pcm := toMono(raw)
	c.mono[path] = pcm
	return pcm, nil
}

func (c *sourceCache) loadNormalized(path string, sampleRate int) (*audio.PCM, error) {
	if sampleRate <= 0 {
		return c.loadMono(path)
	}
	c.ensureMaps()
	key := sourceCacheKey{path: path, sampleRate: sampleRate}
	if pcm, ok := c.normalized[key]; ok {
		return pcm, nil
	}
	mono, err := c.loadMono(path)
	if err != nil {
		return nil, err
	}
	if mono.SampleRate == sampleRate {
		c.normalized[key] = mono
		return mono, nil
	}
	pcm := resampleRate(mono, sampleRate)
	c.normalized[key] = pcm
	return pcm, nil
}

func toMono(pcm *audio.PCM) *audio.PCM {
	if pcm.Channels == 1 {
		return pcm
	}
	frames := len(pcm.Data) / pcm.Channels
	data := make([]int16, frames)
	for frame := 0; frame < frames; frame++ {
		sum := 0
		for channel := 0; channel < pcm.Channels; channel++ {
			sum += int(pcm.Data[frame*pcm.Channels+channel])
		}
		data[frame] = int16(sum / pcm.Channels)
	}
	return &audio.PCM{SampleRate: pcm.SampleRate, Channels: 1, Data: data}
}

func resampleRate(pcm *audio.PCM, targetRate int) *audio.PCM {
	frames := len(pcm.Data)
	targetFrames := int(math.Round(float64(frames) * float64(targetRate) / float64(pcm.SampleRate)))
	return &audio.PCM{SampleRate: targetRate, Channels: 1, Data: floatPCM(linearResample(pcmFloats(pcm.Data), targetFrames))}
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

func smoothstep(value float64) float64 {
	value = math.Max(0, math.Min(1, value))
	return value * value * (3 - 2*value)
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

func pcmFloats(data []int16) []float64 {
	result := make([]float64, len(data))
	for i, value := range data {
		result[i] = float64(value) / 32768
	}
	return result
}

func floatPCM(data []float64) []int16 {
	result := make([]int16, len(data))
	for i, value := range data {
		value = math.Max(-1, math.Min(1, value))
		result[i] = int16(math.Round(value * 32767))
	}
	return result
}

func msToFrames(ms float64, sampleRate int) int {
	if ms <= 0 {
		return 0
	}
	return int(math.Round(ms * float64(sampleRate) / 1000))
}

func msToFramesSigned(ms float64, sampleRate int) int {
	return int(math.Round(ms * float64(sampleRate) / 1000))
}
