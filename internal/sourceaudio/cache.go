package sourceaudio

import (
	"container/list"
	"os"
	"path/filepath"
	"sync"

	"utautts/internal/acoustic"
	"utautts/internal/audio"
)

type cachedSource struct {
	path          string
	size, modTime int64
	pcm           *audio.PCM
	mono          []float64
}

type cache struct {
	mu       sync.Mutex
	byPath   map[string]*list.Element
	order    *list.List
	bytes    int64
	maxBytes int64
}

func newCache(limit int64) *cache {
	return &cache{byPath: make(map[string]*list.Element), order: list.New(), maxBytes: limit}
}

// アプリ内の原音専用。生PCMと解析用モノラルを合わせて256MiBまで保持する。
var shared = newCache(256 << 20)

// ReadWavとReadMonoが返す配列は共有・読取専用。編集する場合は呼び出し側で複製する。
func ReadWav(path string) (*audio.PCM, error) {
	entry, err := shared.load(path)
	if err != nil {
		return nil, err
	}
	return entry.pcm, nil
}

func ReadMono(path string) (*audio.PCM, []float64, error) {
	return shared.readMono(path)
}

func TrimmedMono(path string, offset, blank float64) (*audio.PCM, []float64, error) {
	pcm, wave, err := ReadMono(path)
	if err != nil {
		return nil, nil, err
	}
	start, end, err := audio.TrimFrames(pcm, offset, blank)
	if err != nil {
		return nil, nil, err
	}
	return &audio.PCM{SampleRate: pcm.SampleRate, Channels: pcm.Channels,
		Data: pcm.Data[start*pcm.Channels : end*pcm.Channels]}, wave[start:end], nil
}

func Clear() {
	shared.mu.Lock()
	defer shared.mu.Unlock()
	shared.byPath = make(map[string]*list.Element)
	shared.order.Init()
	shared.bytes = 0
}

func (c *cache) remove(element *list.Element) {
	entry := element.Value.(*cachedSource)
	c.bytes -= int64(len(entry.pcm.Data))*2 + int64(len(entry.mono))*8
	delete(c.byPath, entry.path)
	c.order.Remove(element)
}

func (c *cache) evict() {
	for c.bytes > c.maxBytes && c.order.Len() > 0 {
		c.remove(c.order.Back())
	}
}

func (c *cache) load(path string) (*cachedSource, error) {
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	c.mu.Lock()
	defer c.mu.Unlock()
	if element, ok := c.byPath[path]; ok {
		entry := element.Value.(*cachedSource)
		if err == nil && entry.size == info.Size() && entry.modTime == info.ModTime().UnixNano() {
			c.order.MoveToFront(element)
			return entry, nil
		}
		c.remove(element)
	}
	if err != nil {
		return nil, err
	}
	pcm, err := audio.ReadWav(path)
	if err != nil {
		return nil, err
	}
	entry := &cachedSource{path: path, size: info.Size(), modTime: info.ModTime().UnixNano(), pcm: pcm}
	c.byPath[path] = c.order.PushFront(entry)
	c.bytes += int64(len(pcm.Data)) * 2
	c.evict()
	return entry, nil
}

func (c *cache) readMono(path string) (*audio.PCM, []float64, error) {
	entry, err := c.load(path)
	if err != nil {
		return nil, nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if entry.mono == nil {
		entry.mono = acoustic.Mono(entry.pcm)
		if element := c.byPath[entry.path]; element != nil && element.Value.(*cachedSource) == entry {
			c.bytes += int64(len(entry.mono)) * 8
			c.evict()
		}
	}
	return entry.pcm, entry.mono, nil
}
