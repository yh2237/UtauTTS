package worldrender

import (
	"container/list"
	"fmt"
)

const worldFramePeriodMS = FramePeriodMS

// unitD4CThresholdは原音接続の解析に使うD4Cのしきい値。0にして有声判定を使わない。
// WORLDの既定の0.85では、足立レイの「い」のような定常の母音が無声と判定され、雑音になる。
const unitD4CThreshold = 0.0

type worldFeatures struct {
	Frames        int
	FFTSize       int
	F0            []float64
	Spectrum      []float64
	Aperiodicity  []float64
	FramePeriodMS float64
}

func (features worldFeatures) periodMS() float64 {
	if features.FramePeriodMS > 0 {
		return features.FramePeriodMS
	}
	return worldFramePeriodMS
}

type worldEngine interface {
	Close() error
	Analyze([]float64, int, []float64) (worldFeatures, error)
	Synthesize(worldFeatures, int) ([]float64, error)
}

type worldFeatureCacheEntry struct {
	key   string
	value cachedWorldUnit
	bytes int64
}

type worldFeatureCache struct {
	capacity int
	maxBytes int64
	bytes    int64
	order    *list.List
	entries  map[string]*list.Element
}

func newWorldFeatureCache(capacity int) *worldFeatureCache {
	return newWorldFeatureCacheWithLimit(capacity, 256<<20)
}

func newWorldFeatureCacheWithLimit(capacity int, maxBytes int64) *worldFeatureCache {
	return &worldFeatureCache{
		capacity: max(1, capacity), maxBytes: max(1, maxBytes),
		order: list.New(), entries: make(map[string]*list.Element),
	}
}

func (cache *worldFeatureCache) get(key string) (cachedWorldUnit, bool) {
	if cache == nil {
		return cachedWorldUnit{}, false
	}
	element, found := cache.entries[key]
	if !found {
		return cachedWorldUnit{}, false
	}
	cache.order.MoveToFront(element)
	return element.Value.(worldFeatureCacheEntry).value, true
}

func (cache *worldFeatureCache) put(key string, value cachedWorldUnit) {
	if cache == nil {
		return
	}
	if element, found := cache.entries[key]; found {
		previous := element.Value.(worldFeatureCacheEntry)
		entryBytes := worldFeatureCacheBytes(value)
		cache.bytes += entryBytes - previous.bytes
		element.Value = worldFeatureCacheEntry{key: key, value: value, bytes: entryBytes}
		cache.order.MoveToFront(element)
		cache.evict()
		return
	}
	entryBytes := worldFeatureCacheBytes(value)
	element := cache.order.PushFront(worldFeatureCacheEntry{key: key, value: value, bytes: entryBytes})
	cache.entries[key] = element
	cache.bytes += entryBytes
	cache.evict()
}

func (cache *worldFeatureCache) evict() {
	for cache.order.Len() > cache.capacity || cache.bytes > cache.maxBytes {
		oldest := cache.order.Back()
		if oldest == nil {
			return
		}
		entry := oldest.Value.(worldFeatureCacheEntry)
		cache.bytes -= entry.bytes
		delete(cache.entries, entry.key)
		cache.order.Remove(oldest)
	}
}

func worldFeatureCacheBytes(value cachedWorldUnit) int64 {
	features := value.features
	return int64(len(features.F0)+len(features.Spectrum)+len(features.Aperiodicity)) * 8
}

func worldSynthesisLengthAt(frames, sampleRate int, frameMS float64) int {
	if frames < 2 || sampleRate <= 0 {
		return 0
	}
	return int(float64(frames-1)*frameMS/1000*float64(sampleRate)) + 1
}

func validateWorldFeatures(features worldFeatures) error {
	bins := features.FFTSize/2 + 1
	if features.Frames < 2 || features.FFTSize <= 0 || len(features.F0) != features.Frames ||
		len(features.Spectrum) != features.Frames*bins || len(features.Aperiodicity) != features.Frames*bins {
		return fmt.Errorf("invalid WORLD feature shape")
	}
	return nil
}
