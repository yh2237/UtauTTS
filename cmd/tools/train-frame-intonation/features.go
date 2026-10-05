package main

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"utautts/cmd/tools/internal/toolutil"
)

type token struct {
	Mora           string  `json:"mora"`
	Vowel          string  `json:"vowel"`
	Pause          bool    `json:"pause"`
	Start          float64 `json:"start_ms"`
	End            float64 `json:"end_ms"`
	Duration       float64 `json:"duration_ms"`
	AccentPosition int     `json:"accent_phrase_position"`
	AccentLength   int     `json:"accent_phrase_length"`
	AccentNucleus  int     `json:"accent_nucleus"`
	AccentHigh     bool    `json:"accent_high"`
	AccentStart    bool    `json:"accent_phrase_start"`
	AccentEnd      bool    `json:"accent_phrase_end"`
	WordStart      bool    `json:"word_start"`
	WordEnd        bool    `json:"word_end"`
	POS            string  `json:"pos"`
	POSGroup       string  `json:"pos_group1"`
	Language       string  `json:"language"`
	Phones         []struct {
		Symbol string `json:"symbol"`
		Role   string `json:"role"`
	} `json:"phones"`
	StressKnown bool `json:"stress_known"`
	Stress      int  `json:"stress"`
	WordIndex   int  `json:"word_index"`
}
type record struct {
	Version         int     `json:"version"`
	ID              string  `json:"id"`
	RecordID        string  `json:"record_id"`
	Text            string  `json:"text"`
	AudioPath       string  `json:"audio_path"`
	AlignmentSource string  `json:"alignment_source"`
	Tokens          []token `json:"tokens"`
	Start           float64 `json:"start_ms"`
	End             float64 `json:"end_ms"`
	MedianF0        float64 `json:"median_f0_hz"`
	Language        string  `json:"language"`
	Speaker         string  `json:"speaker"`
	Split           string  `json:"split"`
}
type featureValue struct {
	Frame, Column int
	Value         float32
}
type example struct {
	ID      string
	Sparse  []featureValue
	Targets []float32
	Mask    []bool
	Frames  int
}

func loadRecords(path string) ([]record, error) {
	var rows []record
	err := toolutil.ScanJSONL(path, func(line []byte) error {
		var r record
		if e := json.Unmarshal(line, &r); e != nil {
			return e
		}
		if r.Version != 1 || r.ID == "" || r.AudioPath == "" || len(r.Tokens) == 0 {
			return fmt.Errorf("invalid version-1 record %q", r.ID)
		}
		rows = append(rows, r)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("empty dataset")
	}
	return rows, nil
}
func fnv1a(s string) uint32 {
	v := uint32(2166136261)
	for _, b := range []byte(s) {
		v ^= uint32(b)
		v *= 16777619
	}
	return v
}
func splitRecords(rows []record, holdout bool) (train, valid, test []record) {
	for _, r := range rows {
		if r.Language == "en" {
			switch r.Split {
			case "train":
				train = append(train, r)
			case "validation":
				valid = append(valid, r)
			case "test":
				test = append(test, r)
			}
			continue
		}
		switch fnv1a(r.ID) % 10 {
		case 0:
			valid = append(valid, r)
		case 1:
			if holdout {
				test = append(test, r)
			} else {
				train = append(train, r)
			}
		default:
			train = append(train, r)
		}
	}
	return
}
func validateEnglishSplits(rows []record) error {
	seen := map[string]string{}
	for _, r := range rows {
		if r.Speaker == "" || r.Split != "train" && r.Split != "validation" && r.Split != "test" {
			return fmt.Errorf("English record %s needs speaker and explicit train/validation/test split", r.ID)
		}
		for _, key := range []string{"speaker:" + r.Speaker, "text:" + r.Text} {
			if prior, ok := seen[key]; ok && prior != r.Split {
				return fmt.Errorf("English split leakage at %s: %s in %s and %s", r.ID, key, prior, r.Split)
			}
			seen[key] = r.Split
		}
	}
	return nil
}
func timeGrid(r record, step float64) []float64 {
	start, end := r.Start, r.End
	if end <= start {
		start = r.Tokens[0].Start
		end = r.Tokens[len(r.Tokens)-1].End
	}
	n := max(1, int(math.Ceil((end-start)/step)))
	out := make([]float64, n)
	for i := range out {
		out[i] = start + (float64(i)+.5)*step
	}
	return out
}
func tokenAt(ts []token, t float64) int {
	for i, x := range ts {
		end := math.Max(x.Start+1e-6, x.End)
		if x.Start <= t && t < end {
			return i
		}
	}
	if t < ts[0].Start {
		return 0
	}
	return len(ts) - 1
}
func tokenFeatures(ts []token, i int, english bool) map[string]float64 {
	t := ts[i]
	p := float64(i) / float64(max(1, len(ts)-1))
	m := map[string]float64{"bias": 1, "position": p, "position2": p * p, "from_end": 1 - p}
	if i == 0 || ts[i-1].Pause {
		m["phrase_start"] = 1
	}
	if i == len(ts)-1 || ts[i+1].Pause {
		m["phrase_end"] = 1
	}
	if english {
		add := func(prefix string, x token) {
			if x.Pause {
				m[prefix+"=<PAUSE>"] = 1
				return
			}
			var syms []string
			for _, ph := range x.Phones {
				syms = append(syms, ph.Symbol)
				m[prefix+"_"+ph.Role+"="+ph.Symbol] = 1
			}
			m[prefix+"="+strings.Join(syms, " ")] = 1
			if x.StressKnown {
				m[fmt.Sprintf("%s_stress=%d", prefix, x.Stress)] = 1
			}
		}
		add("syllable", t)
		if i > 0 {
			add("prev", ts[i-1])
		} else {
			m["prev=<BOS>"] = 1
		}
		if i+1 < len(ts) {
			add("next", ts[i+1])
		} else {
			m["next=<EOS>"] = 1
		}
		if !t.Pause {
			if i == 0 || ts[i-1].Pause || ts[i-1].WordIndex != t.WordIndex {
				m["en_word_start"] = 1
			}
			if t.WordEnd {
				m["en_word_end"] = 1
			}
		}
		return m
	}
	add := func(prefix string, x token) {
		if x.Pause {
			m[prefix+"=<PAUSE>"] = 1
		} else {
			m[prefix+"="+x.Mora] = 1
			m[prefix+"_vowel="+x.Vowel] = 1
		}
	}
	add("mora", t)
	if i > 0 {
		add("prev", ts[i-1])
	} else {
		m["prev=<BOS>"] = 1
	}
	if i+1 < len(ts) {
		add("next", ts[i+1])
	} else {
		m["next=<EOS>"] = 1
	}
	length := t.AccentLength
	if length == 0 {
		length = len(ts)
	}
	length = max(1, length)
	pos := t.AccentPosition
	if pos == 0 {
		pos = i + 1
	}
	den := float64(length)
	m["accent_position"] = float64(pos) / den
	m["accent_from_end"] = float64(length-pos) / den
	m["accent_nucleus_position"] = float64(t.AccentNucleus) / den
	if t.AccentHigh {
		m["accent_high"] = 1
	} else {
		m["accent_high"] = 0
	}
	if t.AccentStart || (t.AccentLength == 0 && i == 0) {
		m["accent_phrase_start"] = 1
	} else {
		m["accent_phrase_start"] = 0
	}
	if t.AccentEnd || (t.AccentLength == 0 && i == len(ts)-1) {
		m["accent_phrase_end"] = 1
	} else {
		m["accent_phrase_end"] = 0
	}
	if t.WordStart {
		m["word_start"] = 1
	} else {
		m["word_start"] = 0
	}
	if t.WordEnd {
		m["word_end"] = 1
	} else {
		m["word_end"] = 0
	}
	posName, group := t.POS, t.POSGroup
	if posName == "" {
		posName = "*"
	}
	if group == "" {
		group = "*"
	}
	m["pos="+posName] = 1
	m["pos_group1="+group] = 1
	kind := "heiban"
	if t.AccentNucleus > 0 {
		if pos < t.AccentNucleus {
			kind = "before"
		} else if pos == t.AccentNucleus {
			kind = "nucleus"
		} else {
			kind = "after"
		}
	}
	m["accent_type="+kind] = 1
	return m
}
func frameFeatures(r record, i int, t, start, end float64) map[string]float64 {
	m := tokenFeatures(r.Tokens, i, r.Language == "en")
	x := r.Tokens[i]
	duration := math.Max(1, x.End-x.Start)
	progress := math.Max(0, math.Min(1, (t-x.Start)/duration))
	position := math.Max(0, math.Min(1, (t-start)/math.Max(1, end-start)))
	m["mora_progress"] = progress
	m["mora_progress2"] = progress * progress
	m["frame_position"] = position
	m["frame_from_end"] = 1 - position
	m["final_distance"] = 1 - position
	m["question_distance"] = 0
	if strings.ContainsAny(r.Text, "?？") {
		m["question_distance"] = 1 - position
	}
	return m
}
func featureNames(rows []record, step float64) []string {
	set := map[string]bool{}
	for _, r := range rows {
		times := timeGrid(r, step)
		start, end := times[0]-step*.5, times[len(times)-1]+step*.5
		for _, t := range times {
			for k := range frameFeatures(r, tokenAt(r.Tokens, t), t, start, end) {
				set[k] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
func npyF64(path string) ([]float64, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	var h [10]byte
	if _, e = io.ReadFull(f, h[:]); e != nil {
		return nil, e
	}
	if string(h[:6]) != "\x93NUMPY" {
		return nil, fmt.Errorf("invalid NPY")
	}
	n := int(binary.LittleEndian.Uint16(h[8:]))
	if h[6] >= 2 {
		var extra [2]byte
		if _, e = io.ReadFull(f, extra[:]); e != nil {
			return nil, e
		}
		n = int(uint32(h[8]) | uint32(h[9])<<8 | uint32(extra[0])<<16 | uint32(extra[1])<<24)
	}
	header := make([]byte, n)
	if _, e = io.ReadFull(f, header); e != nil {
		return nil, e
	}
	if !strings.Contains(string(header), "<f8") {
		return nil, fmt.Errorf("NPY must contain little-endian float64")
	}
	data, e := io.ReadAll(f)
	if e != nil {
		return nil, e
	}
	if len(data)%8 != 0 {
		return nil, fmt.Errorf("NPY size")
	}
	out := make([]float64, len(data)/8)
	for i := range out {
		out[i] = math.Float64frombits(binary.LittleEndian.Uint64(data[i*8:]))
	}
	return out, nil
}
func pyFloat(v float64) string {
	s := strconv.FormatFloat(math.Round(v*1000)/1000, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}
func cachePath(dir string, r record, step float64) string {
	return cachePathTag(dir, r, step, "internal_autocorrelation")
}
func cachePathTag(dir string, r record, step float64, tag string) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, t := range r.Tokens {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("[" + pyFloat(t.Start) + ", " + pyFloat(t.End) + ", " + map[bool]string{true: "true", false: "false"}[t.Pause] + "]")
	}
	b.WriteByte(']')
	id := r.RecordID
	if id == "" {
		id = r.ID
	}
	key := fmt.Sprintf("%s|%g|%s|%s", id, step, tag, b.String())
	h := sha1.Sum([]byte(key))
	return filepath.Join(dir, fmt.Sprintf("%x.npy", h))
}
