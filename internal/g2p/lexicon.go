package g2p

import (
	"bufio"
	"bytes"
	"compress/gzip"
	_ "embed"
	"fmt"
	"strings"
	"sync"
)

// Sourceは読みをどこから得たか。
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

// Notationは読みの書き方。
type Notation string

const (
	NotationARPAbet Notation = "arpabet"
	NotationKokoro  Notation = "kokoro"
)

// Readingは1語の読みと、その出所。
type Reading struct {
	Phones     string
	Notation   Notation
	Source     Source
	Confidence float64
}

// EnglishEntryは英語辞書の1項目。
type EnglishEntry struct {
	Phones string
	Source Source
}

//go:embed lexicon/cmudict.dict.gz
var cmuDictionary []byte

//go:embed lexicon/librispeech.dict.gz
var libriSpeechDictionary []byte

var englishLexiconOnce sync.Once
var cmuWords map[string]string
var cmuError error
var libriSpeechOnce sync.Once
var libriSpeechWords map[string]string
var libriSpeechError error

// LookupEnglishはCMUdictを先に引き、無い語だけLibriSpeechの辞書を引く。
func LookupEnglish(word string) (EnglishEntry, error) {
	englishLexiconOnce.Do(func() {
		cmuWords, cmuError = loadWordList(cmuDictionary)
		if cmuError == nil && len(cmuWords) < 100000 {
			cmuError = fmt.Errorf("incomplete embedded English dictionary")
		}
	})
	if cmuError != nil {
		return EnglishEntry{}, cmuError
	}
	key := strings.ToLower(word)
	if phones := cmuWords[key]; phones != "" {
		return EnglishEntry{Phones: phones, Source: SourceCMU}, nil
	}
	phones, err := lookupLibriSpeech(key)
	if err != nil {
		// 予備の辞書が読めないときは、G2Pや綴り規則へ進む。
		return EnglishEntry{}, nil
	}
	if phones == "" {
		return EnglishEntry{}, nil
	}
	return EnglishEntry{Phones: phones, Source: SourceLibriSpeech}, nil
}

func lookupLibriSpeech(word string) (string, error) {
	libriSpeechOnce.Do(func() {
		libriSpeechWords, libriSpeechError = loadWordList(libriSpeechDictionary)
		if libriSpeechError == nil && len(libriSpeechWords) < 100000 {
			libriSpeechError = fmt.Errorf("incomplete embedded LibriSpeech dictionary")
		}
	})
	if libriSpeechError != nil {
		return "", libriSpeechError
	}
	return libriSpeechWords[word], nil
}

func loadWordList(data []byte) (map[string]string, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	words := make(map[string]string, 140000)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		word, phones, ok := strings.Cut(line, "\t")
		if ok {
			if word == "" || phones == "" {
				continue
			}
			words[word] = phones
			continue
		}
		// CMUdictは「語 音素...」の空白区切りで、#以降は注釈。
		line, _, _ = strings.Cut(line, "#")
		fields := strings.Fields(line)
		if len(fields) < 2 || strings.Contains(fields[0], "(") {
			continue
		}
		words[strings.ToLower(fields[0])] = strings.Join(fields[1:], " ")
	}
	return words, scanner.Err()
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
