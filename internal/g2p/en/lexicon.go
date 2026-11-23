package en

import (
	"bufio"
	"bytes"
	"compress/gzip"
	_ "embed"
	"fmt"
	"strings"
	"sync"

	"github.com/yh2237/UtauTTS/internal/g2p"
)

type EnglishEntry struct {
	Phones string
	Source g2p.Source
}

//go:embed lexicon/cmudict.dict.gz
var cmuDictionary []byte

//go:embed lexicon/librispeech.dict.gz
var libriSpeechDictionary []byte

var (
	englishLexiconOnce sync.Once
	cmuWords           map[string]string
	cmuError           error
	libriSpeechOnce    sync.Once
	libriSpeechWords   map[string]string
	libriSpeechError   error
)

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
		return EnglishEntry{Phones: phones, Source: g2p.SourceCMU}, nil
	}
	phones, err := lookupLibriSpeech(key)
	if err != nil {
		// 予備の辞書が読めないときは、G2Pや綴り規則へ進む。
		return EnglishEntry{}, nil
	}
	if phones == "" {
		return EnglishEntry{}, nil
	}
	return EnglishEntry{Phones: phones, Source: g2p.SourceLibriSpeech}, nil
}

func lookupEnglishDictionary(word string) (string, error) {
	entry, err := LookupEnglish(word)
	return entry.Phones, err
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
