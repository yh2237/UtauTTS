package frontend

import (
	"fmt"
	"strings"
	"sync"

	"github.com/NK8007/gofonix/pkg/g2p"
)

var (
	englishG2POnce   sync.Once
	englishG2PEngine *g2p.Engine
	englishG2PErr    error
)

func englishWordPronunciation(word string) (string, error) {
	englishG2POnce.Do(func() {
		englishG2PEngine, englishG2PErr = g2p.New(g2p.Options{Language: "en", Mode: g2p.ModeBatch})
	})
	if englishG2PErr != nil {
		return "", englishG2PErr
	}
	result, err := englishG2PEngine.Process(word)
	if err != nil {
		return "", err
	}
	var symbols []string
	for _, token := range result.Tokens {
		for _, phoneme := range token.Pronunciation.Phonemes {
			symbol := arpabetByPhonemeID[phoneme.ID]
			if symbol == "" {
				return "", fmt.Errorf("unknown English phoneme ID %d", phoneme.ID)
			}
			symbols = append(symbols, symbol)
		}
	}
	if len(symbols) == 0 {
		return "", fmt.Errorf("cannot infer English pronunciation for %q", word)
	}
	return strings.Join(symbols, " "), nil
}

var arpabetByPhonemeID = map[int]string{
	1: "AA", 2: "AE", 3: "AH", 4: "AO", 5: "AW", 6: "AY",
	7: "B", 8: "CH", 9: "D", 10: "DH", 11: "EH", 12: "ER",
	13: "EY", 14: "F", 15: "G", 16: "HH", 17: "M", 18: "IY",
	19: "JH", 20: "K", 21: "L", 22: "IH", 23: "N", 24: "NG",
	25: "OW", 26: "OY", 27: "P", 28: "R", 29: "S", 30: "SH",
	31: "T", 32: "TH", 33: "UH", 34: "UW", 35: "V", 36: "W",
	37: "Y", 38: "Z", 39: "ZH",
}
