package base

import (
	"container/list"
	"context"
	"fmt"
	"math"
	"os"
	"sync"

	"utautts/internal/audio"
)

// SourceCacheは音源録音のデコード結果をレンダリング中に再利用する。
type SourceCache struct {
	raw        map[string]*audio.PCM
	mono       map[string]*audio.PCM
	normalized map[sourceCacheKey]*audio.PCM
}

type sourceCacheKey struct {
	path       string
	sampleRate int
}

// NewSourceCacheは空のSourceCacheを作る。
func NewSourceCache() SourceCache {
	return SourceCache{
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

// LoadMonoは音源をモノラルで読み込む。
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

// LoadNormalizedは音源を指定サンプルレートへ揃えて読み込む。
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

func framesToMS(frames, sampleRate int) float64 {
	if sampleRate <= 0 {
		return 0
	}
	return float64(frames) * 1000 / float64(sampleRate)
}

func smoothstep(value float64) float64 {
	value = math.Max(0, math.Min(1, value))
	return value * value * (3 - 2*value)
}

// Smoothstepは0..1へクランプした滑らかな補間値を返す。
func Smoothstep(value float64) float64 { return smoothstep(value) }

// MsToFramesはミリ秒をフレーム数へ丸める。
func MsToFrames(ms float64, sampleRate int) int { return msToFrames(ms, sampleRate) }

// MsToFramesSignedは符号付きミリ秒をフレーム数へ丸める。
func MsToFramesSigned(ms float64, sampleRate int) int { return msToFramesSigned(ms, sampleRate) }

// FramesToMSはフレーム数をミリ秒へ変換する。
func FramesToMS(frames, sampleRate int) float64 { return framesToMS(frames, sampleRate) }

// PcmFloatsはint16 PCMを-1..1のfloatへ変換する。
func PcmFloats(data []int16) []float64 { return pcmFloats(data) }

// FloatPCMは-1..1のfloatをint16 PCMへ変換する。
func FloatPCM(data []float64) []int16 { return floatPCM(data) }

// ToMonoは多チャンネルPCMをモノラルへ変換する。
func ToMono(pcm *audio.PCM) *audio.PCM { return toMono(pcm) }

// ResampleRateはPCMを指定サンプルレートへ変換する。
func ResampleRate(pcm *audio.PCM, targetRate int) *audio.PCM { return resampleRate(pcm, targetRate) }

// ContextErrorはコンテキストのキャンセルをrenderer共通のエラーへ変換する。
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
