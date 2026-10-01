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
	monoOnce      sync.Once
	mono          []float64
}

// 同じ原音・同じ更新状態の初回読込を1回にまとめる。
type pendingLoad struct {
	size, modTime int64
	done          chan struct{}
	entry         *cachedSource
	err           error
}

type cache struct {
	mu         sync.Mutex
	byPath     map[string]*list.Element
	loading    map[string]*pendingLoad
	order      *list.List
	bytes      int64
	maxBytes   int64
	generation uint64
}

func newCache(limit int64) *cache {
	return &cache{byPath: make(map[string]*list.Element), loading: make(map[string]*pendingLoad),
		order: list.New(), maxBytes: limit}
}

// 試験で読込の開始・完了を制御するために差し替える。
var readWavFile = audio.ReadWav

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
	// 消去前に始まった読込の結果は保持しない。
	shared.generation++
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

// ファイルの読込・復号はロックの外で行い、他の原音の取得を待たせない。
func (c *cache) load(path string) (*cachedSource, error) {
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	c.mu.Lock()
	if element, ok := c.byPath[path]; ok {
		entry := element.Value.(*cachedSource)
		if err == nil && entry.size == info.Size() && entry.modTime == info.ModTime().UnixNano() {
			c.order.MoveToFront(element)
			c.mu.Unlock()
			return entry, nil
		}
		c.remove(element)
	}
	if err != nil {
		c.mu.Unlock()
		return nil, err
	}
	size, modTime := info.Size(), info.ModTime().UnixNano()
	if pending := c.loading[path]; pending != nil && pending.size == size && pending.modTime == modTime {
		c.mu.Unlock()
		<-pending.done
		return pending.entry, pending.err
	}
	pending := &pendingLoad{size: size, modTime: modTime, done: make(chan struct{})}
	c.loading[path] = pending
	generation := c.generation
	c.mu.Unlock()

	pcm, err := readWavFile(path)
	if err == nil {
		pending.entry = &cachedSource{path: path, size: size, modTime: modTime, pcm: pcm}
	}
	pending.err = err

	c.mu.Lock()
	if c.loading[path] == pending {
		delete(c.loading, path)
	}
	if err == nil && generation == c.generation {
		if element, ok := c.byPath[path]; ok {
			c.remove(element)
		}
		c.byPath[path] = c.order.PushFront(pending.entry)
		c.bytes += int64(len(pcm.Data)) * 2
		c.evict()
	}
	c.mu.Unlock()
	close(pending.done)
	return pending.entry, pending.err
}

func (c *cache) readMono(path string) (*audio.PCM, []float64, error) {
	entry, err := c.load(path)
	if err != nil {
		return nil, nil, err
	}
	entry.monoOnce.Do(func() {
		mono := acoustic.Mono(entry.pcm)
		c.mu.Lock()
		entry.mono = mono
		if element := c.byPath[entry.path]; element != nil && element.Value.(*cachedSource) == entry {
			c.bytes += int64(len(mono)) * 8
			c.evict()
		}
		c.mu.Unlock()
	})
	return entry.pcm, entry.mono, nil
}
