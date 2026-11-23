package g2p

import "sync"

type Source string

const (
	SourceUser        Source = "user"
	SourceEntity      Source = "entity"
	SourceMisaki      Source = "misaki"
	SourceCMU         Source = "cmu"
	SourceLibriSpeech Source = "librispeech"
	SourceHeteronym   Source = "heteronym"
	SourceMorphology  Source = "morphology"
	SourceCompound    Source = "compound"
	SourceAcronym     Source = "acronym"
	SourceNeural      Source = "neural"
	SourceRules       Source = "rules"
)

type Notation string

const (
	NotationARPAbet Notation = "arpabet"
	NotationKokoro  Notation = "kokoro"
)

type Reading struct {
	Phones     string
	Notation   Notation
	Source     Source
	Confidence float64
}

// ReadingCacheは語の読みを覚える。単語数は限られるので、上限を超えたら1件ずつ捨てる。
type ReadingCache struct {
	mu      sync.RWMutex
	entries map[string]Reading
	max     int
}

func NewReadingCache(max int) *ReadingCache {
	if max <= 0 {
		max = 4096
	}
	return &ReadingCache{entries: make(map[string]Reading, max), max: max}
}

func (c *ReadingCache) Get(key string) (Reading, bool) {
	c.mu.RLock()
	reading, ok := c.entries[key]
	c.mu.RUnlock()
	return reading, ok
}

func (c *ReadingCache) Put(key string, reading Reading) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= c.max {
		for existing := range c.entries {
			delete(c.entries, existing)
			break
		}
	}
	c.entries[key] = reading
}
