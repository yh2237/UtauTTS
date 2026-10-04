package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/yh2237/gograd/autograd"
	"utautts/internal/openjtalk"
)

type predictionCase struct {
	ID      string  `json:"id"`
	Text    string  `json:"text"`
	Reading string  `json:"reading"`
	Tokens  []token `json:"tokens"`
}

func loadPredictionCases(path string) ([]predictionCase, error) {
	data, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	if strings.EqualFold(filepath.Ext(path), ".jsonl") {
		var rows []predictionCase
		s := bufio.NewScanner(strings.NewReader(string(data)))
		s.Buffer(make([]byte, 4096), 16<<20)
		for s.Scan() {
			if strings.TrimSpace(s.Text()) == "" {
				continue
			}
			var r predictionCase
			if e = json.Unmarshal(s.Bytes(), &r); e != nil {
				return nil, e
			}
			rows = append(rows, r)
		}
		return rows, s.Err()
	}
	var rows []predictionCase
	if e = json.Unmarshal(data, &rows); e == nil {
		return rows, nil
	}
	var wrapper struct {
		Cases []predictionCase `json:"cases"`
	}
	if e = json.Unmarshal(data, &wrapper); e != nil {
		return nil, e
	}
	return wrapper.Cases, nil
}

func fallbackPredictionTokens(text string) []token {
	var out []token
	for _, char := range text {
		if unicode.IsSpace(char) {
			continue
		}
		pause := strings.ContainsRune(",.、。", char)
		start := float64(len(out)) * 100
		out = append(out, token{Mora: string(char), Pause: pause, Start: start, End: start + 100, Duration: 100})
	}
	if len(out) == 0 {
		out = append(out, token{End: 100, Duration: 100})
	}
	ensurePredictionAccent(out)
	return out
}

func ensurePredictionAccent(tokens []token) {
	for start := 0; start < len(tokens); {
		if tokens[start].Pause {
			start++
			continue
		}
		end := start + 1
		for end < len(tokens) && !tokens[end].Pause {
			end++
		}
		for i := start; i < end; i++ {
			if tokens[i].AccentLength == 0 {
				tokens[i].AccentLength = end - start
				tokens[i].AccentPosition = i - start + 1
				tokens[i].AccentHigh = i > start
				tokens[i].AccentStart = i == start
				tokens[i].AccentEnd = i == end-1
				tokens[i].WordStart = i == start
				tokens[i].WordEnd = i == end-1
			}
		}
		start = end
	}
}

func predictCorpus(model *tcn, path string, index map[string]int, c config) (map[string]any, error) {
	items, e := loadPredictionCases(path)
	if e != nil {
		return nil, e
	}
	cases := make([]map[string]any, 0, len(items))
	for k, item := range items {
		if len(item.Tokens) == 0 {
			if c.Language != "ja" {
				return nil, fmt.Errorf("case %d: English raw text needs timed tokens", k)
			}
			a, err := openjtalk.Analyze(item.Text, openjtalk.Config{HelperPath: c.OpenJTalkHelper, DictionaryPath: c.OpenJTalkDictionary})
			if err != nil {
				item.Tokens = fallbackPredictionTokens(item.Text)
			} else {
				item.Reading = a.Reading
				for i, mora := range a.Morae {
					item.Tokens = append(item.Tokens, token{Mora: mora, Pause: len(a.Features[i]) == 0})
				}
				row := record{ID: "prediction", Text: item.Text, Tokens: item.Tokens}
				applyAnalysis(&row, a)
				item.Tokens = row.Tokens
			}
		}
		if c.Language == "ja" {
			ensurePredictionAccent(item.Tokens)
		}
		if item.Reading == "" {
			item.Reading = item.Text
		}
		cursor := 0.0
		for i := range item.Tokens {
			t := &item.Tokens[i]
			d := t.Duration
			if d <= 0 {
				d = 100
				if t.Pause {
					d = 120
				}
			}
			if t.Start == 0 && i > 0 {
				t.Start = cursor
			}
			if t.End <= t.Start {
				t.End = t.Start + d
			}
			cursor = math.Max(cursor, t.End)
		}
		if len(item.Tokens) == 0 {
			return nil, fmt.Errorf("case %d has no tokens", k)
		}
		n := max(1, int(math.Ceil(cursor/c.Frame)))
		r := record{Text: item.Text, Language: c.Language, Tokens: item.Tokens}
		ex := example{Frames: n, Mask: make([]bool, n)}
		for i := 0; i < n; i++ {
			t := (float64(i) + .5) * c.Frame
			j := tokenAt(r.Tokens, t)
			ex.Mask[i] = !r.Tokens[j].Pause
			for name, v := range frameFeatures(r, j, t, 0, float64(n)*c.Frame) {
				if col, ok := index[name]; ok && v != 0 {
					ex.Sparse = append(ex.Sparse, featureValue{Frame: i, Column: col, Value: float32(v)})
				}
			}
		}
		var predicted []float32
		var evalErr error
		autograd.NoGrad(func() {
			x, y, _, _, err := batchExamples([]example{ex}, []int{0}, len(index), model.Device)
			if err != nil {
				evalErr = err
				return
			}
			p := model.forward(x)
			predicted, evalErr = p.ToHost()
			p.ReleaseGraph()
			x.Close()
			y.Close()
		})
		if evalErr != nil {
			return nil, evalErr
		}
		centered := make([]float64, 0, n)
		for i, v := range predicted {
			if ex.Mask[i] {
				centered = append(centered, float64(v))
			}
		}
		med := lowerMedian(centered)
		scale := math.Max(1, math.Max(math.Abs(c.Low), math.Abs(c.High)))
		cents := make([]float64, n)
		for i, v := range predicted {
			value := math.Max(c.Low, math.Min(c.High, (float64(v)-med)*scale))
			cents[i] = math.Round(value*1e6) / 1e6
		}
		id := item.ID
		if id == "" {
			id = fmt.Sprintf("case-%04d", k)
		}
		cases = append(cases, map[string]any{"id": id, "text": item.Text, "reading": item.Reading, "frame_ms": c.Frame, "cents": cents})
	}
	return map[string]any{"version": 1, "name": "intonation-frame-tcn-v8", "cases": cases}, nil
}
