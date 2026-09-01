package main

import (
	"container/list"
	"fmt"
)

const worldFramePeriodMS = 10.0

type worldFeatures struct {
	Frames       int
	FFTSize      int
	F0           []float64
	Spectrum     []float64
	Aperiodicity []float64
}

type worldEngine interface {
	Close() error
	Analyze([]float64, int, []float64) (worldFeatures, error)
	Synthesize(worldFeatures, int) ([]float64, error)
}

type worldFeatureCacheEntry struct {
	key   string
	value cachedWorldUnit
}

type worldFeatureCache struct {
	capacity int
	order    *list.List
	entries  map[string]*list.Element
}

func newWorldFeatureCache(capacity int) *worldFeatureCache {
	return &worldFeatureCache{capacity: max(1, capacity), order: list.New(), entries: make(map[string]*list.Element)}
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
		element.Value = worldFeatureCacheEntry{key: key, value: value}
		cache.order.MoveToFront(element)
		return
	}
	element := cache.order.PushFront(worldFeatureCacheEntry{key: key, value: value})
	cache.entries[key] = element
	if cache.order.Len() <= cache.capacity {
		return
	}
	oldest := cache.order.Back()
	delete(cache.entries, oldest.Value.(worldFeatureCacheEntry).key)
	cache.order.Remove(oldest)
}

func worldSynthesisLength(frames, sampleRate int) int {
	if frames < 2 || sampleRate <= 0 {
		return 0
	}
	return int(float64(frames-1)*worldFramePeriodMS/1000*float64(sampleRate)) + 1
}

func validateWorldFeatures(features worldFeatures) error {
	bins := features.FFTSize/2 + 1
	if features.Frames < 2 || features.FFTSize <= 0 || len(features.F0) != features.Frames ||
		len(features.Spectrum) != features.Frames*bins || len(features.Aperiodicity) != features.Frames*bins {
		return fmt.Errorf("invalid WORLD feature shape")
	}
	return nil
}
