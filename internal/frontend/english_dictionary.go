package frontend

import (
	"bufio"
	"bytes"
	"compress/gzip"
	_ "embed"
	"fmt"
	"strings"
	"sync"
)

//go:embed lexicon/cmudict.dict.gz
var cmuDictionary []byte

var cmuOnce sync.Once
var cmuWords map[string]string
var cmuError error

func lookupEnglishDictionary(word string) (string, error) {
	cmuOnce.Do(func() {
		reader, err := gzip.NewReader(bytes.NewReader(cmuDictionary))
		if err != nil {
			cmuError = err
			return
		}
		defer reader.Close()
		cmuWords = make(map[string]string)
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			line, _, _ := strings.Cut(scanner.Text(), "#")
			fields := strings.Fields(line)
			if len(fields) < 2 || strings.Contains(fields[0], "(") {
				continue
			}
			cmuWords[strings.ToLower(fields[0])] = strings.Join(fields[1:], " ")
		}
		cmuError = scanner.Err()
		if cmuError == nil && len(cmuWords) < 100000 {
			cmuError = fmt.Errorf("incomplete embedded English dictionary")
		}
	})
	return cmuWords[strings.ToLower(word)], cmuError
}
