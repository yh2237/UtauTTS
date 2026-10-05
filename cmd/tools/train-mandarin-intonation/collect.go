package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/parquet-go/parquet-go"
	"utautts/cmd/tools/internal/toolutil"
	"utautts/internal/audio"
)

type parquetRecord struct {
	Audio struct {
		Bytes []byte `parquet:"bytes"`
		Path  string `parquet:"path"`
	} `parquet:"audio"`
	Pinyin string `parquet:"pinyin"`
}

type syllable struct {
	start, end float64
	label      string
}

var intervalPattern = regexp.MustCompile(`intervals \[\d+\]:\s*xmin = ([\d.]+)\s*xmax = ([\d.]+)\s*text = "([^"]*)"`)

func textgridSyllables(path string) ([]syllable, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	start := strings.Index(string(raw), `name = "words"`)
	if start < 0 {
		return nil, fmt.Errorf("words tier absent: %s", path)
	}
	section := string(raw[start:])
	if end := strings.Index(section, "item [2]:"); end >= 0 {
		section = section[:end]
	}
	matches := intervalPattern.FindAllStringSubmatch(section, -1)
	out := make([]syllable, 0, len(matches))
	for _, m := range matches {
		a, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return nil, err
		}
		b, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			return nil, err
		}
		if m[3] != "" {
			out = append(out, syllable{a, b, m[3]})
		}
	}
	return out, nil
}

func median(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	sort.Float64s(v)
	n := len(v)
	if n%2 == 1 {
		return v[n/2]
	}
	return (v[n/2-1] + v[n/2]) / 2
}

func sampledF0(f0 []float64, start, end, progress float64) float64 {
	center := start + (end-start)*progress
	first := max(0, int(math.RoundToEven((center-.025)*100)))
	last := min(len(f0), int(math.RoundToEven((center+.025)*100))+1)
	var voiced []float64
	for i := first; i < last; i++ {
		if f0[i] > 55 && f0[i] < 650 {
			voiced = append(voiced, f0[i])
		}
	}
	return median(voiced)
}

func tonePoints(tone int, final bool, previous int) [][2]float64 {
	switch tone {
	case 1:
		return [][2]float64{{0, 145}, {1, 145}}
	case 2:
		return [][2]float64{{0, -20}, {.3, -55}, {1, 145}}
	case 3:
		if final {
			return [][2]float64{{0, -35}, {.55, -145}, {1, 65}}
		}
		return [][2]float64{{0, -35}, {.65, -145}, {1, -115}}
	case 4:
		return [][2]float64{{0, 145}, {.2, 115}, {1, -145}}
	default:
		ends := []float64{0, -100, -55, 65, -130, -35}
		end := -35.0
		if previous >= 1 && previous <= 5 {
			end = ends[previous]
		}
		return [][2]float64{{0, end + 25}, {1, end}}
	}
}

func baseline(tone int, final bool, previous int, progress float64) float64 {
	p := tonePoints(tone, final, previous)
	for i := 0; i+1 < len(p); i++ {
		if progress <= p[i+1][0] {
			ratio := max(0, min(1, (progress-p[i][0])/(p[i+1][0]-p[i][0])))
			ratio = ratio * ratio * (3 - 2*ratio)
			return p[i][1] + (p[i+1][1]-p[i][1])*ratio
		}
	}
	return p[len(p)-1][1]
}

func toneFeatures(tones []int, index int, starts, ends []bool) []float64 {
	x := make([]float64, len(features))
	position := float64(index) / float64(max(1, len(tones)-1))
	x[0], x[1], x[2] = 1, position, position*position
	if starts[index] {
		x[3] = 1
	}
	if ends[index] {
		x[4] = 1
	}
	x[4+tones[index]] = 1
	if index > 0 && !starts[index] {
		x[9+tones[index-1]] = 1
	}
	if index+1 < len(tones) && !ends[index] {
		x[14+tones[index+1]] = 1
	}
	return x
}

func rowFromRecord(rec parquetRecord, alignments, worldEngine, cache string) (row, string, error) {
	utterance := strings.TrimSuffix(filepath.Base(rec.Audio.Path), filepath.Ext(rec.Audio.Path))
	speaker := utterance[:min(7, len(utterance))]
	grid := filepath.Join(alignments, speaker, utterance+".TextGrid")
	intervals, err := textgridSyllables(grid)
	if os.IsNotExist(err) {
		return row{}, "missing_alignment", nil
	}
	if err != nil {
		return row{}, "", err
	}
	labels := strings.Fields(rec.Pinyin)
	if len(labels) != len(intervals) || len(labels) < 3 {
		return row{}, "label_mismatch", nil
	}
	tones := make([]int, len(labels))
	for i, label := range labels {
		if label != intervals[i].label {
			return row{}, "label_mismatch", nil
		}
		if len(label) == 0 || label[len(label)-1] < '1' || label[len(label)-1] > '5' {
			return row{}, "invalid_tone", nil
		}
		tones[i] = int(label[len(label)-1] - '0')
	}
	pcm, err := audio.DecodeWav(bytes.NewReader(rec.Audio.Bytes))
	if err != nil {
		return row{}, "", fmt.Errorf("decode %s: %w", utterance, err)
	}
	samples := make([]float64, len(pcm.Data)/pcm.Channels)
	for i := range samples {
		for c := 0; c < pcm.Channels; c++ {
			samples[i] += float64(pcm.Data[i*pcm.Channels+c]) / 32768
		}
		samples[i] /= float64(pcm.Channels)
	}
	var f0 []float64
	key := fmt.Sprintf("%s-%x.json", utterance, sha256.Sum256(rec.Audio.Bytes))
	cachePath := filepath.Join(cache, key)
	if raw, e := os.ReadFile(cachePath); e == nil {
		if e = json.Unmarshal(raw, &f0); e != nil {
			return row{}, "", e
		}
	} else {
		f0, err = worldF0(samples, pcm.SampleRate, 10, worldEngine)
		if err != nil {
			return row{}, "", fmt.Errorf("WORLD %s: %w", utterance, err)
		}
		if err = os.MkdirAll(cache, 0755); err != nil {
			return row{}, "", err
		}
		raw, err := json.Marshal(f0)
		if err != nil {
			return row{}, "", err
		}
		if err = os.WriteFile(cachePath, raw, 0644); err != nil {
			return row{}, "", err
		}
	}
	starts, ends := make([]bool, len(tones)), make([]bool, len(tones))
	measured := make([][]float64, len(tones))
	valid := make([][]bool, len(tones))
	allMeasured := []float64{}
	for i := range tones {
		starts[i] = i == 0 || intervals[i].start-intervals[i-1].end > .12
		ends[i] = i == len(tones)-1 || intervals[i+1].start-intervals[i].end > .12
		measured[i], valid[i] = make([]float64, len(knots)), make([]bool, len(knots))
		for k, progress := range knots {
			measured[i][k] = sampledF0(f0, intervals[i].start, intervals[i].end, progress)
			valid[i][k] = measured[i][k] > 0
			if valid[i][k] {
				allMeasured = append(allMeasured, measured[i][k])
			}
		}
	}
	if len(allMeasured) < max(4, len(tones)) {
		return row{}, "unvoiced", nil
	}
	center := median(allMeasured)
	r := row{Speaker: speaker, Utterance: utterance, Valid: valid}
	r.X, r.Residual, r.Rule, r.LogF0 = make([][]float64, len(tones)), make([][]float64, len(tones)), make([][]float64, len(tones)), make([][]float64, len(tones))
	allRule := []float64{}
	for i := range tones {
		r.X[i] = toneFeatures(tones, i, starts, ends)
		r.Residual[i], r.Rule[i], r.LogF0[i] = make([]float64, len(knots)), make([]float64, len(knots)), make([]float64, len(knots))
		previous := 5
		if i > 0 && !starts[i] {
			previous = tones[i-1]
		}
		for k, progress := range knots {
			r.Rule[i][k] = baseline(tones[i], ends[i], previous, progress)
			if valid[i][k] {
				r.LogF0[i][k] = 1200 * math.Log2(measured[i][k]/center)
				allRule = append(allRule, r.Rule[i][k])
			}
		}
	}
	ruleCenter := median(allRule)
	for i := range tones {
		for k := range knots {
			r.Rule[i][k] -= ruleCenter
			r.Residual[i][k] = r.LogF0[i][k] - r.Rule[i][k]
		}
	}
	return r, "", nil
}

func collect(parquetPath, alignments, worldEngine, cache string, limit, workers int) ([]row, map[string]int, map[string]int, error) {
	f, err := os.Open(parquetPath)
	if err != nil {
		return nil, nil, nil, err
	}
	defer f.Close()
	reader := parquet.NewGenericReader[parquetRecord](f)
	defer reader.Close()
	counts, skipped := map[string]int{}, map[string]int{}
	var rows []row
	batch := make([]parquetRecord, workers*2)
	for {
		n, err := reader.Read(batch)
		if err != nil && err != io.EOF {
			return nil, nil, nil, err
		}
		if n == 0 {
			break
		}
		type result struct {
			row     row
			reason  string
			err     error
			speaker string
		}
		results := make([]result, n)
		var wg sync.WaitGroup
		semaphore := make(chan struct{}, workers)
		for i := 0; i < n; i++ {
			rec := batch[i]
			utterance := strings.TrimSuffix(filepath.Base(rec.Audio.Path), filepath.Ext(rec.Audio.Path))
			speaker := utterance[:min(7, len(utterance))]
			results[i].speaker = speaker
			if counts[speaker] >= limit {
				continue
			}
			wg.Add(1)
			semaphore <- struct{}{}
			go func(i int, rec parquetRecord) {
				defer wg.Done()
				defer func() { <-semaphore }()
				results[i].row, results[i].reason, results[i].err = rowFromRecord(rec, alignments, worldEngine, cache)
			}(i, rec)
		}
		wg.Wait()
		for _, got := range results {
			if counts[got.speaker] >= limit {
				continue
			}
			if got.err != nil {
				return nil, nil, nil, got.err
			}
			if got.reason != "" {
				skipped[got.reason]++
			} else {
				rows = append(rows, got.row)
				counts[got.speaker]++
			}
		}
		if err == io.EOF {
			break
		}
	}
	return rows, counts, skipped, nil
}

func writeRows(path string, rows []row) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := toolutil.CreateExclusive(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, r := range rows {
		if err = enc.Encode(r); err != nil {
			return err
		}
	}
	return nil
}
