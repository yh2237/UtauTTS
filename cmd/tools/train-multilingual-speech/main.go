// train-multilingual-speech fits aligned natural-speech phone corrections.
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type phone struct {
	Symbol   string    `json:"symbol"`
	Features []string  `json:"features"`
	Baseline float64   `json:"baseline_ms"`
	Duration float64   `json:"duration_ms"`
	Pitch    []float64 `json:"pitch_cents"`
	Energy   *float64  `json:"energy_log_ratio"`
}
type record struct {
	Version                                                                        int `json:"version"`
	FeatureVersion                                                                 int `json:"feature_version"`
	ID, Language, Speaker, Split, Kind, Alignment, Corpus, License, AudioSHA, Text string
	Phones                                                                         []phone `json:"phones"`
}

func (r *record) UnmarshalJSON(data []byte) error {
	type alias record
	var row struct {
		Version        int     `json:"version"`
		FeatureVersion int     `json:"feature_version"`
		ID             string  `json:"id"`
		Language       string  `json:"language"`
		Speaker        string  `json:"speaker"`
		Split          string  `json:"split"`
		Kind           string  `json:"kind"`
		Alignment      string  `json:"alignment"`
		Corpus         string  `json:"corpus"`
		License        string  `json:"license"`
		AudioSHA       string  `json:"audio_sha256"`
		Text           string  `json:"text"`
		Phones         []phone `json:"phones"`
	}
	if e := json.Unmarshal(data, &row); e != nil {
		return e
	}
	*r = record{row.Version, row.FeatureVersion, row.ID, row.Language, row.Speaker, row.Split, row.Kind, row.Alignment, row.Corpus, row.License, row.AudioSHA, row.Text, row.Phones}
	return nil
}
func load(path string) ([]record, string, error) {
	raw, e := os.ReadFile(path)
	if e != nil {
		return nil, "", e
	}
	hash := sha256.Sum256(raw)
	s := bufio.NewScanner(strings.NewReader(strings.TrimPrefix(string(raw), "\ufeff")))
	s.Buffer(make([]byte, 4096), 16<<20)
	var rows []record
	for s.Scan() {
		if strings.TrimSpace(s.Text()) == "" {
			continue
		}
		var r record
		if e := json.Unmarshal(s.Bytes(), &r); e != nil {
			return nil, "", e
		}
		rows = append(rows, r)
	}
	return rows, fmt.Sprintf("%x", hash), s.Err()
}
func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }
func validate(rows []record, language string) error {
	if len(rows) == 0 || (language != "en" && language != "zh") {
		return fmt.Errorf("aligned en/zh records required")
	}
	ids := map[string]bool{}
	splits := map[string]string{}
	hasTrain, hasValid := false, false
	for _, r := range rows {
		if r.Version != 1 || r.FeatureVersion != 1 || r.Language != language {
			return fmt.Errorf("unsupported record schema or mixed languages")
		}
		if r.Kind != "natural" || (r.Alignment != "manual" && r.Alignment != "forced") {
			return fmt.Errorf("unaligned/generated data cannot be natural-speech training labels")
		}
		if r.ID == "" || r.Speaker == "" || r.Corpus == "" || r.License == "" || r.Text == "" || r.AudioSHA == "" {
			return fmt.Errorf("missing provenance")
		}
		if ids[r.ID] || (r.Split != "train" && r.Split != "validation" && r.Split != "test") {
			return fmt.Errorf("duplicate ID or missing split")
		}
		ids[r.ID] = true
		for _, key := range []string{"speaker:" + r.Speaker, "audio:" + r.AudioSHA, "text:" + r.Text} {
			if prior, ok := splits[key]; ok && prior != r.Split {
				return fmt.Errorf("train/held-out leakage: %s", key)
			}
			splits[key] = r.Split
		}
		if len(r.Phones) == 0 {
			return fmt.Errorf("empty phone record")
		}
		hasTrain = hasTrain || r.Split == "train"
		hasValid = hasValid || r.Split == "validation"
		for _, p := range r.Phones {
			if p.Symbol == "" || len(p.Features) == 0 {
				return fmt.Errorf("invalid phone features")
			}
			seen := map[string]bool{}
			for _, f := range p.Features {
				if seen[f] {
					return fmt.Errorf("invalid phone features")
				}
				seen[f] = true
			}
			if !seen["bias"] {
				return fmt.Errorf("invalid phone features")
			}
			if !finite(p.Baseline) || p.Baseline <= 0 || !finite(p.Duration) || p.Duration <= 0 {
				return fmt.Errorf("invalid measured duration")
			}
			if p.Pitch != nil {
				if len(p.Pitch) != 3 {
					return fmt.Errorf("invalid pitch knots")
				}
				for _, v := range p.Pitch {
					if !finite(v) {
						return fmt.Errorf("invalid pitch knots")
					}
				}
			}
			if p.Energy != nil && !finite(*p.Energy) {
				return fmt.Errorf("invalid energy")
			}
		}
	}
	if !hasTrain || !hasValid {
		return fmt.Errorf("separate training and validation speakers required")
	}
	return nil
}
func solveRidge(rows []phone, names []string, target func(phone) (float64, bool), ridge float64) map[string]float64 {
	var chosen []phone
	var values []float64
	for _, p := range rows {
		if v, ok := target(p); ok {
			chosen = append(chosen, p)
			values = append(values, v)
		}
	}
	if len(chosen) < 5 {
		return nil
	}
	n := len(names)
	index := map[string]int{}
	for i, name := range names {
		index[name] = i
	}
	a := make([][]float64, n)
	for i := range a {
		a[i] = make([]float64, n)
		a[i][i] = ridge
	}
	a[0][0] = ridge // multilingual Python regularizes bias equally.
	b := make([]float64, n)
	for row, p := range chosen {
		ids := make([]int, 0, len(p.Features))
		for _, f := range p.Features {
			if i, ok := index[f]; ok {
				ids = append(ids, i)
			}
		}
		for _, i := range ids {
			b[i] += values[row]
			for _, j := range ids {
				a[i][j]++
			}
		}
	}
	// Positive ridge makes A positive definite. Cholesky avoids a new module dependency.
	l := make([][]float64, n)
	for i := range l {
		l[i] = make([]float64, n)
		for j := 0; j <= i; j++ {
			v := a[i][j]
			for k := 0; k < j; k++ {
				v -= l[i][k] * l[j][k]
			}
			if i == j {
				l[i][j] = math.Sqrt(v)
			} else {
				l[i][j] = v / l[j][j]
			}
		}
	}
	y := make([]float64, n)
	for i := range y {
		v := b[i]
		for j := 0; j < i; j++ {
			v -= l[i][j] * y[j]
		}
		y[i] = v / l[i][i]
	}
	w := make([]float64, n)
	for i := n - 1; i >= 0; i-- {
		v := y[i]
		for j := i + 1; j < n; j++ {
			v -= l[j][i] * w[j]
		}
		w[i] = v / l[i][i]
	}
	out := map[string]float64{}
	for i, name := range names {
		out[name] = w[i]
	}
	return out
}
func mean(xs []float64) any {
	if len(xs) == 0 {
		return nil
	}
	sum := 0.0
	for _, v := range xs {
		sum += v
	}
	return sum / float64(len(xs))
}
func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }
func sumHead(head map[string]float64, features []string) float64 {
	sum := 0.0
	for _, f := range features {
		sum += head[f]
	}
	return sum
}
func train(rows []record, language, id string, ridge float64) (map[string]any, error) {
	if e := validate(rows, language); e != nil {
		return nil, e
	}
	if !finite(ridge) || ridge <= 0 {
		return nil, fmt.Errorf("positive finite ridge regularization required")
	}
	var training []phone
	corpora, licenses := map[string]bool{}, map[string]bool{}
	for _, r := range rows {
		corpora[r.Corpus] = true
		licenses[r.License] = true
		if r.Split == "train" {
			training = append(training, r.Phones...)
		}
	}
	featureSet := map[string]bool{}
	counts := map[string]int{}
	for _, p := range training {
		counts[strings.ToLower(p.Symbol)]++
		for _, f := range p.Features {
			featureSet[f] = true
		}
	}
	names := make([]string, 0, len(featureSet))
	for f := range featureSet {
		names = append(names, f)
	}
	sort.Strings(names)
	duration := solveRidge(training, names, func(p phone) (float64, bool) { return math.Log(p.Duration / p.Baseline), true }, ridge)
	if duration == nil {
		return nil, fmt.Errorf("at least five training phones required")
	}
	energy := solveRidge(training, names, func(p phone) (float64, bool) {
		if p.Energy == nil {
			return 0, false
		}
		return *p.Energy, true
	}, ridge)
	pitch := make([]map[string]float64, 3)
	hasPitch := true
	for k := range pitch {
		index := k
		pitch[k] = solveRidge(training, names, func(p phone) (float64, bool) {
			if p.Pitch == nil {
				return 0, false
			}
			return p.Pitch[index], true
		}, ridge)
		if pitch[k] == nil {
			hasPitch = false
		}
	}
	pitchCounts := map[string]int{}
	if hasPitch {
		for _, p := range training {
			if p.Pitch != nil {
				pitchCounts[strings.ToLower(p.Symbol)]++
			}
		}
	}
	join := func(m map[string]bool) string {
		v := make([]string, 0, len(m))
		for s := range m {
			v = append(v, s)
		}
		sort.Strings(v)
		return strings.Join(v, "; ")
	}
	model := map[string]any{"version": 1, "feature_version": 1, "id": id, "language": language, "corpus": join(corpora), "license": join(licenses), "training_data_kind": "natural", "phone_counts": counts, "duration_log_ratio": duration, "status": "experimental-requires-listening"}
	if energy != nil {
		model["energy_log_ratio"] = energy
	}
	if hasPitch {
		model["pitch_cents"] = pitch
		model["pitch_phone_counts"] = pitchCounts
	}
	metrics := map[string]any{}
	for _, split := range []string{"validation", "test"} {
		var held []phone
		for _, r := range rows {
			if r.Split == split {
				held = append(held, r.Phones...)
			}
		}
		if len(held) == 0 {
			continue
		}
		baselineErr, modelErr, pitchErr, energyErr := []float64{}, []float64{}, []float64{}, []float64{}
		covered := 0
		for _, p := range held {
			known := counts[strings.ToLower(p.Symbol)] >= 5
			if known {
				covered++
			}
			residual := 0.0
			if known {
				residual = sumHead(duration, p.Features)
			}
			prediction := p.Baseline
			if known {
				prediction = clamp(p.Baseline*math.Exp(clamp(residual, math.Log(.5), math.Log(2))), 8, 500)
			}
			baselineErr = append(baselineErr, math.Abs(p.Duration-p.Baseline))
			modelErr = append(modelErr, math.Abs(p.Duration-prediction))
			if hasPitch && pitchCounts[strings.ToLower(p.Symbol)] >= 5 && p.Pitch != nil {
				for k := 0; k < 3; k++ {
					pitchErr = append(pitchErr, math.Abs(clamp(sumHead(pitch[k], p.Features), -300, 300)-p.Pitch[k]))
				}
			}
			if known && energy != nil && p.Energy != nil {
				energyErr = append(energyErr, math.Abs(clamp(sumHead(energy, p.Features), math.Log(.7), math.Log(1.3))-*p.Energy))
			}
		}
		metrics[split] = map[string]any{"phones": len(held), "covered_phones": covered, "baseline_duration_mae_ms": mean(baselineErr), "model_duration_mae_ms": mean(modelErr), "pitch_mae_cents": mean(pitchErr), "energy_log_mae": mean(energyErr)}
	}
	model["evaluation"] = metrics
	return model, nil
}
func main() {
	args := os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		args = append(append([]string{}, args[1:]...), args[0])
	}
	language := flag.String("language", "", "en or zh")
	id := flag.String("id", "", "model ID")
	out := flag.String("out", "", "new JSON under out/")
	ridge := flag.Float64("ridge", 10, "positive ridge regularization")
	if e := flag.CommandLine.Parse(args); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(2)
	}
	if flag.NArg() != 1 || *language == "" || *id == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "usage: train-multilingual-speech CORPUS --language en|zh --id ID --out out/model.json")
		os.Exit(2)
	}
	root, _ := filepath.Abs("out")
	target, _ := filepath.Abs(*out)
	rel, e := filepath.Rel(root, target)
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		fmt.Fprintln(os.Stderr, "output must be under out/")
		os.Exit(2)
	}
	if _, e = os.Stat(*out); e == nil {
		fmt.Fprintln(os.Stderr, "refusing to overwrite", *out)
		os.Exit(1)
	}
	rows, hash, e := load(flag.Arg(0))
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	model, e := train(rows, *language, *id, *ridge)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	model["training_manifest_sha256"] = hash
	if e = os.MkdirAll(filepath.Dir(*out), 0755); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	b, e := json.MarshalIndent(model, "", "  ")
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	f, e := os.OpenFile(*out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	_, e = f.Write(append(b, '\n'))
	closeError := f.Close()
	if e == nil {
		e = closeError
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	summary, _ := json.MarshalIndent(model["evaluation"], "", "  ")
	fmt.Println(string(summary))
}
