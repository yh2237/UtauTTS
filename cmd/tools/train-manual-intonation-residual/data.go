package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"utautts/internal/frontend"
	"utautts/internal/openjtalk"
)

type phrase struct {
	ID string
	X  []map[string]float64
	Y  []float64
}

var extra = []string{"base_pitch_cents", "base_prev_cents", "base_next_cents", "base_delta_prev", "base_delta_next", "base_second_difference", "base_phrase_min", "base_phrase_max", "base_phrase_range", "base_near_render_limit"}

func asMap(v any) map[string]any { x, _ := v.(map[string]any); return x }
func asList(v any) []any         { x, _ := v.([]any); return x }
func asString(v any) string      { x, _ := v.(string); return x }
func asBool(v any) bool          { x, _ := v.(bool); return x }
func asFloat(v any) float64      { x, _ := v.(float64); return x }
func numbers(v any) []float64 {
	a := asList(v)
	out := make([]float64, len(a))
	for i, x := range a {
		value := asFloat(x)
		if math.IsNaN(value) || math.IsInf(value, 0) {
			value = 0
		}
		out[i] = value
	}
	return out
}
func at(v []float64, ms, t float64) float64 {
	if len(v) == 0 || ms <= 0 {
		return 0
	}
	p := math.Max(0, t/ms)
	a := min(len(v)-1, int(p))
	b := min(len(v)-1, a+1)
	return v[a]*(float64(b)-p) + v[b]*(p-float64(a))
}
func timing(e map[string]any, morae []string, pauses []bool) ([]float64, []float64) {
	d, p := numbers(e["mora_durations_ms"]), numbers(e["mora_positions_ms"])
	if len(d) != len(morae) {
		d = numbers(e["automatic_mora_durations_ms"])
	}
	if len(d) != len(morae) {
		d = make([]float64, len(morae))
		for i := range d {
			d[i] = 120
			if pauses[i] {
				d[i] = 180
			}
		}
	}
	for i := range d {
		d[i] = math.Max(1, d[i])
	}
	if len(p) != len(morae) {
		p = numbers(e["automatic_mora_positions_ms"])
	}
	if len(p) != len(morae) {
		p = make([]float64, len(morae))
		cursor := 0.0
		for i, x := range d {
			p[i] = cursor
			cursor += x
		}
	}
	return p, d
}
func phraseIDs(features []map[string]float64, pauses []bool) []int {
	ids := make([]int, len(pauses))
	q := -1
	for i := range ids {
		if pauses[i] {
			ids[i] = -1
			continue
		}
		if i == 0 || pauses[i-1] || features[i]["accent_phrase_start"] == 1 {
			q++
		}
		ids[i] = q
	}
	return ids
}
func tokenFeatures(morae, vowels []string, pauses []bool, base []map[string]float64, i int) map[string]float64 {
	n := len(morae)
	p := float64(i) / float64(max(1, n-1))
	f := map[string]float64{"bias": 1, "position": p, "position2": p * p, "from_end": 1 - p}
	if i == 0 || pauses[i-1] {
		f["phrase_start"] = 1
	}
	if i == n-1 || pauses[i+1] {
		f["phrase_end"] = 1
	}
	add := func(prefix string, j int) {
		if j < 0 {
			f[prefix+"=<BOS>"] = 1
			return
		}
		if j >= n {
			f[prefix+"=<EOS>"] = 1
			return
		}
		if pauses[j] {
			f[prefix+"=<PAUSE>"] = 1
			return
		}
		f[prefix+"="+morae[j]] = 1
		f[prefix+"_vowel="+vowels[j]] = 1
	}
	add("mora", i)
	add("prev", i-1)
	add("next", i+1)
	for k, v := range base[i] {
		f[k] = v
	}
	return f
}
func featureRows(a *openjtalk.Analysis, base []float64) ([]map[string]float64, []int, error) {
	if len(a.Morae) != len(base) {
		return nil, nil, fmt.Errorf("base mora count differs")
	}
	pauses := make([]bool, len(base))
	kana, err := frontend.ParseKana(a.Reading)
	if err != nil || len(kana) != len(base) {
		return nil, nil, fmt.Errorf("Open JTalk reading mora count differs")
	}
	vowels := make([]string, len(base))
	for i := range kana {
		vowels[i] = kana[i].Vowel
	}
	features := make([]map[string]float64, len(base))
	for i, f := range a.Features {
		pauses[i] = len(f) == 0
		features[i] = map[string]float64{}
		for k, v := range f {
			features[i][k] = v
		}
	}
	ids := phraseIDs(features, pauses)
	bounds := map[int][2]float64{}
	for i, id := range ids {
		if id < 0 {
			continue
		}
		b, ok := bounds[id]
		if !ok {
			b = [2]float64{base[i], base[i]}
		}
		b[0] = math.Min(b[0], base[i])
		b[1] = math.Max(b[1], base[i])
		bounds[id] = b
	}
	rows := make([]map[string]float64, len(base))
	for i := range rows {
		prev, next := base[i], base[i]
		if i > 0 && ids[i-1] == ids[i] {
			prev = base[i-1]
		}
		if i+1 < len(base) && ids[i+1] == ids[i] {
			next = base[i+1]
		}
		x := tokenFeatures(a.Morae, vowels, pauses, features, i)
		x["base_pitch_cents"] = base[i] / 120
		x["base_prev_cents"] = prev / 120
		x["base_next_cents"] = next / 120
		x["base_delta_prev"] = (base[i] - prev) / 120
		x["base_delta_next"] = (next - base[i]) / 120
		x["base_second_difference"] = (next - 2*base[i] + prev) / 120
		x["base_near_render_limit"] = math.Abs(base[i]) / 90
		if ids[i] >= 0 {
			b := bounds[ids[i]]
			x["base_phrase_min"] = b[0] / 120
			x["base_phrase_max"] = b[1] / 120
			x["base_phrase_range"] = (b[1] - b[0]) / 120
		}
		rows[i] = x
	}
	return rows, ids, nil
}
func loadPhrases(paths []string, limit float64, cfg openjtalk.Config) ([]phrase, int, []string, error) {
	var out []phrase
	accepted := 0
	var skipped []string
	for _, path := range paths {
		b, e := os.ReadFile(path)
		if e != nil {
			return nil, 0, nil, e
		}
		var session map[string]any
		if e = json.Unmarshal(b, &session); e != nil {
			return nil, 0, nil, e
		}
		for pos, item := range asList(session["utterances"]) {
			entry := asMap(item)
			if !asBool(entry["training_accepted"]) {
				continue
			}
			accepted++
			id := fmt.Sprintf("%s:%v", filepath.Base(path), entry["lab_entry_id"])
			if entry["lab_entry_id"] == nil {
				id = fmt.Sprintf("%s:%d", filepath.Base(path), pos)
			}
			a, e := openjtalk.Analyze(asString(entry["text"]), cfg)
			if e != nil {
				skipped = append(skipped, id+": "+e.Error())
				continue
			}
			cache := asList(asMap(entry["analysis_cache"])["morae"])
			if len(cache) != len(a.Morae) {
				skipped = append(skipped, id+": Open JTalk mora count differs")
				continue
			}
			valid := true
			pauses := make([]bool, len(cache))
			for i, raw := range cache {
				m := asMap(raw)
				pauses[i] = asBool(m["pause"])
				if pauses[i] != (len(a.Features[i]) == 0) || (!pauses[i] && asString(m["mora"]) != a.Morae[i]) {
					valid = false
					break
				}
			}
			if !valid {
				skipped = append(skipped, id+": mora differs")
				continue
			}
			ms := asFloat(entry["automatic_frame_ms"])
			if ms == 0 {
				ms = 10
			}
			auto := numbers(entry["automatic_frame_pitch"])
			if ms <= 0 || len(auto) < 2 {
				skipped = append(skipped, id+": automatic frame contour is missing")
				continue
			}
			starts, durations := timing(entry, a.Morae, pauses)
			base := make([]float64, len(cache))
			for i := range base {
				base[i] = at(auto, ms, starts[i]+durations[i]/2)
			}
			frames, points := numbers(entry["pitch_frames"]), numbers(entry["pitch_points"])
			targets := make([]float64, len(cache))
			for i := range targets {
				z := 0.0
				if len(frames) > 0 {
					begin := max(0, int(math.Ceil(starts[i]/ms)))
					end := min(len(frames), int(math.Ceil((starts[i]+durations[i])/ms)))
					if begin < end {
						values := append([]float64(nil), frames[begin:end]...)
						sort.Float64s(values)
						z = values[len(values)/2]
					} else {
						z = at(frames, ms, starts[i]+durations[i]/2)
					}
				} else if i < len(points) {
					z = points[i]
				}
				targets[i] = math.Max(-limit, math.Min(limit, z))
			}
			rows, ids, e := featureRows(a, base)
			if e != nil {
				skipped = append(skipped, id+": "+e.Error())
				continue
			}
			for q := 0; q < len(ids); q++ {
				if ids[q] != q { /* phrase IDs need not equal indices */
				}
			}
			groups := map[int]*phrase{}
			order := []int{}
			for i, pid := range ids {
				if pid < 0 {
					continue
				}
				g, ok := groups[pid]
				if !ok {
					g = &phrase{ID: id}
					groups[pid] = g
					order = append(order, pid)
				}
				g.X = append(g.X, rows[i])
				g.Y = append(g.Y, targets[i])
			}
			for _, pid := range order {
				out = append(out, *groups[pid])
			}
		}
	}
	return out, accepted, skipped, nil
}
func featureNames(rows []phrase) []string {
	set := map[string]bool{}
	for _, name := range extra {
		set[name] = true
	}
	for _, r := range rows {
		for _, x := range r.X {
			for name := range x {
				set[name] = true
			}
		}
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
func outputPath(path string) error {
	abs, e := filepath.Abs(path)
	if e != nil {
		return e
	}
	root, e := filepath.Abs("out")
	if e != nil {
		return e
	}
	rel, e := filepath.Rel(root, abs)
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("output must be under out/")
	}
	if _, e := os.Stat(path); e == nil {
		return fmt.Errorf("refusing to overwrite %s", path)
	}
	return nil
}
