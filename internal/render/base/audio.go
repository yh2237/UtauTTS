package base

import (
	"container/list"
	"context"
	"fmt"
	"math"
	"sync"

	"github.com/yh2237/UtauIDX/audio"
	"github.com/yh2237/UtauTTS/internal/sourceaudio"
	"github.com/yh2237/audiodsp/stretch"
)

type SourceCache struct {
	raw        map[string]*audio.PCM
	mono       map[string]*audio.PCM
	normalized map[sourceCacheKey]*audio.PCM
}

type sourceCacheKey struct {
	path       string
	sampleRate int
}

func NewSourceCache() SourceCache {
	return SourceCache{
		raw:        make(map[string]*audio.PCM),
		mono:       make(map[string]*audio.PCM),
		normalized: make(map[sourceCacheKey]*audio.PCM),
	}
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

func loadWAVCached(path string) (*audio.PCM, error) {
	return sourceaudio.ReadWav(path)
}

func ClearWAVCache() {
	sourceaudio.Clear()
	globalUnitPitchCache.Lock()
	globalUnitPitchCache.entries = make(map[unitPitchCacheKey]*list.Element)
	globalUnitPitchCache.order.Init()
	globalUnitPitchCache.Unlock()
}

func (c *SourceCache) ensureMaps() {
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

func (c *SourceCache) load(path string) (*audio.PCM, error) {
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

func (c *SourceCache) LoadMono(path string) (*audio.PCM, error) {
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

func (c *SourceCache) LoadNormalized(path string, sampleRate int) (*audio.PCM, error) {
	if sampleRate <= 0 {
		return c.LoadMono(path)
	}
	c.ensureMaps()
	key := sourceCacheKey{path: path, sampleRate: sampleRate}
	if pcm, ok := c.normalized[key]; ok {
		return pcm, nil
	}
	mono, err := c.LoadMono(path)
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

func MsToFrames(ms float64, sampleRate int) int { return msToFrames(ms, sampleRate) }

func MsToFramesSigned(ms float64, sampleRate int) int { return msToFramesSigned(ms, sampleRate) }

func PcmFloats(data []int16) []float64 { return pcmFloats(data) }

func FloatPCM(data []float64) []int16 { return floatPCM(data) }

func ContextError(ctx context.Context) error {
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

func linearResample(source []float64, targetFrames int) []float64 {
	return stretch.Linear(source, targetFrames)
}
