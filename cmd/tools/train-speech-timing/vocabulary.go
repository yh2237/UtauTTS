package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"utautts/cmd/tools/internal/toolutil"
)

const phoneNames = "<pad> <unk> sil a i u e o N cl k g s sh z j t ch ts d n h f b p m y r w v ky gy ny hy my ry by py dy ty"

// vocabularyは学習データの音素記号とIDの対応。言語ごとに異なる。
type vocabulary struct {
	names   []string
	ids     map[string]int
	silence int
	unknown int
}

func newVocabulary(names []string) *vocabulary {
	v := &vocabulary{names: append([]string(nil), names...), ids: make(map[string]int, len(names))}
	for i, name := range names {
		v.ids[name] = i
	}
	v.silence = v.id("sil")
	v.unknown = v.id("<unk>")
	return v
}

func (v *vocabulary) id(name string) int {
	if index, ok := v.ids[name]; ok {
		return index
	}
	return v.unknown
}

func (v *vocabulary) String() string { return strings.Join(v.names, " ") }

func jaVocabulary() *vocabulary { return newVocabulary(strings.Fields(phoneNames)) }

// vocabularyFromSymbolsはデータに現れる記号から語彙を作る。並びは<pad> <unk> silの後に辞書順。
func vocabularyFromSymbols(symbols []string) *vocabulary {
	unique := map[string]bool{}
	for _, symbol := range symbols {
		symbol = strings.TrimSpace(symbol)
		if symbol != "" && symbol != "sil" {
			unique[symbol] = true
		}
	}
	sorted := make([]string, 0, len(unique))
	for symbol := range unique {
		sorted = append(sorted, symbol)
	}
	sort.Strings(sorted)
	return newVocabulary(append([]string{"<pad>", "<unk>", "sil"}, sorted...))
}

type corpusPhone struct {
	Symbol  string  `json:"symbol"`
	StartMS float64 `json:"start_ms"`
	EndMS   float64 `json:"end_ms"`
}

type corpusToken struct {
	Pause   bool          `json:"pause"`
	StartMS float64       `json:"start_ms"`
	EndMS   float64       `json:"end_ms"`
	Phones  []corpusPhone `json:"phones"`
}

type corpusRecord struct {
	ID        string        `json:"id"`
	AudioPath string        `json:"audio_path"`
	Language  string        `json:"language"`
	Tokens    []corpusToken `json:"tokens"`
}

// phonesFromTokensはcorpusのトークン境界を音素列へ変換し、隙間をsilで埋める。
func phonesFromTokens(tokens []corpusToken) []phone {
	var ps []phone
	for _, token := range tokens {
		if token.Pause {
			ps = append(ps, phone{token.StartMS / 1000, token.EndMS / 1000, "sil", "sil"})
			continue
		}
		for _, p := range token.Phones {
			name := strings.TrimSpace(p.Symbol)
			if name == "" {
				name = "sil"
			}
			ps = append(ps, phone{p.StartMS / 1000, p.EndMS / 1000, name, name})
		}
	}
	return fillSilence(ps)
}

func fillSilence(ps []phone) []phone {
	var result []phone
	cursor := 0.0
	for _, p := range ps {
		if p.start > cursor+1e-4 {
			result = append(result, phone{cursor, p.start, "sil", ""})
		}
		result = append(result, p)
		cursor = p.end
	}
	return result
}

func corpusVocabulary(path string) (*vocabulary, error) {
	var symbols []string
	err := toolutil.ScanJSONL(path, func(line []byte) error {
		var r corpusRecord
		if e := json.Unmarshal(line, &r); e != nil {
			return e
		}
		for _, token := range r.Tokens {
			for _, p := range token.Phones {
				symbols = append(symbols, p.Symbol)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(symbols) == 0 {
		return nil, fmt.Errorf("corpus %s has no phone symbols", path)
	}
	return vocabularyFromSymbols(symbols), nil
}
