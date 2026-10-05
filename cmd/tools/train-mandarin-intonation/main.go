package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"utautts/cmd/tools/internal/toolutil"
)

var knots = []float64{.25, .40, .55, .70, .85}
var features = []string{"bias", "position", "position2", "phrase_start", "phrase_end", "tone_1", "tone_2", "tone_3", "tone_4", "tone_5", "prev_tone_1", "prev_tone_2", "prev_tone_3", "prev_tone_4", "prev_tone_5", "next_tone_1", "next_tone_2", "next_tone_3", "next_tone_4", "next_tone_5"}

type row struct {
	Speaker   string      `json:"speaker"`
	Utterance string      `json:"utterance"`
	X         [][]float64 `json:"x"`
	Residual  [][]float64 `json:"residual"`
	Valid     [][]bool    `json:"valid"`
	Rule      [][]float64 `json:"rule"`
	LogF0     [][]float64 `json:"log_f0"`
}

func readRows(path string) ([]row, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 4096), 16<<20)
	var out []row
	for s.Scan() {
		if strings.TrimSpace(s.Text()) == "" {
			continue
		}
		var r row
		if e = json.Unmarshal(s.Bytes(), &r); e != nil {
			return nil, e
		}
		if r.Speaker == "" || len(r.X) == 0 || len(r.X) != len(r.Valid) || len(r.X) != len(r.Residual) || len(r.X) != len(r.Rule) || len(r.X) != len(r.LogF0) {
			return nil, fmt.Errorf("invalid observation row")
		}
		for i := range r.X {
			if len(r.X[i]) != len(features) || len(r.Valid[i]) != len(knots) || len(r.Residual[i]) != len(knots) || len(r.Rule[i]) != len(knots) || len(r.LogF0[i]) != len(knots) {
				return nil, fmt.Errorf("invalid observation dimensions")
			}
		}
		out = append(out, r)
	}
	return out, s.Err()
}
func solve(a [][]float64, b []float64) []float64 {
	n := len(b)
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
	return w
}
func fit(rows []row, lambda float64) [][]float64 {
	weights := make([][]float64, len(knots))
	n := len(features)
	for k := range knots {
		a := make([][]float64, n)
		b := make([]float64, n)
		for i := range a {
			a[i] = make([]float64, n)
			a[i][i] = lambda
		}
		a[0][0] = lambda * .1
		for _, r := range rows {
			for i, x := range r.X {
				if !r.Valid[i][k] || math.Abs(r.Residual[i][k]) >= 600 {
					continue
				}
				y := r.Residual[i][k]
				for p, v := range x {
					b[p] += v * y
					for q, u := range x {
						a[p][q] += v * u
					}
				}
			}
		}
		weights[k] = solve(a, b)
	}
	return weights
}
func dot(x, y []float64) float64 {
	v := 0.0
	for i, a := range x {
		v += a * y[i]
	}
	return v
}
func evaluate(rows []row, weights [][]float64) map[string]any {
	sum, base, count := 0.0, 0.0, 0
	for _, r := range rows {
		for i, x := range r.X {
			for k := range knots {
				if !r.Valid[i][k] {
					continue
				}
				correction := math.Max(-100, math.Min(100, dot(x, weights[k]))) * .65
				sum += math.Abs(r.Rule[i][k] + correction - r.LogF0[i][k])
				base += math.Abs(r.Rule[i][k] - r.LogF0[i][k])
				count++
			}
		}
	}
	if count == 0 {
		return map[string]any{"mae_cents": 0.0, "rule_mae_cents": 0.0, "evaluated_knots": 0}
	}
	return map[string]any{"mae_cents": math.RoundToEven(sum/float64(count)*100) / 100, "rule_mae_cents": math.RoundToEven(base/float64(count)*100) / 100, "evaluated_knots": count}
}
func train(rows []row) (map[string]any, error) {
	return trainWithSkipped(rows, nil)
}
func trainWithSkipped(rows []row, skipped map[string]int) (map[string]any, error) {
	counts := map[string]int{}
	for _, r := range rows {
		counts[r.Speaker]++
	}
	if len(counts) < 3 {
		return nil, fmt.Errorf("at least three speakers are required for a held-out evaluation")
	}
	speakers := make([]string, 0, len(counts))
	for s := range counts {
		speakers = append(speakers, s)
	}
	sort.Strings(speakers)
	heldOut := speakers[len(speakers)-1]
	var training, test []row
	for _, r := range rows {
		if r.Speaker == heldOut {
			test = append(test, r)
		} else {
			training = append(training, r)
		}
	}
	if len(training) < 100 || len(test) < 50 {
		return nil, fmt.Errorf("insufficient training or held-out utterances")
	}
	best := math.Inf(1)
	var lambda float64
	var metrics map[string]any
	for _, candidate := range []float64{25, 100, 400, 1200} {
		w := fit(training, candidate)
		score := evaluate(test, w)
		mae := score["mae_cents"].(float64)
		if mae < best {
			best = mae
			lambda = candidate
			metrics = score
		}
	}
	weights := fit(training, lambda)
	tokens := func(group []row) int {
		n := 0
		for _, r := range group {
			n += len(r.X)
		}
		return n
	}
	if skipped == nil {
		skipped = map[string]int{}
	}
	return map[string]any{"id": "tone-intonation-zh-v1", "display_name": "Mandarin Tone Intonation v1", "description": "AISHELL-3 learned correction to Mandarin tone contours", "license": "Apache License 2.0 (model weights)", "license_notices": []string{"licenses/TONE-INTONATION-ZH-V1.txt", "licenses/AISHELL-3-NOTICE.txt", "licenses/PADDLESPEECH-AISHELL3-ALIGNMENT-NOTICE.txt", "licenses/APACHE-2.0.txt"}, "language": "zh", "provenance": map[string]any{"training_corpus": "AISHELL-3 subset (3 speakers, 750 utterances)", "training_data_kind": "natural-mandarin-speech", "corpus_url": "https://www.openslr.org/93/", "source_license": "Apache License 2.0", "alignment": "PaddleSpeech AISHELL-3 MFA2.x with tone", "alignment_url": "https://github.com/PaddlePaddle/PaddleSpeech/tree/develop/examples/aishell3/tts3"}, "recommended_renderers": []string{"utautts-world-phrase"}, "default_priority": 60, "version": 13, "feature_version": 1, "mode": "mandarin_intonation_v1", "mandarin_intonation": map[string]any{"feature_names": features, "knots": knots, "weights": weights, "strength": .65, "max_cents": 100}, "metrics": map[string]any{"records": len(test), "tokens": tokens(test), "pitch_mae_cents": metrics["mae_cents"], "baseline_pitch_mae_cents": metrics["rule_mae_cents"]}, "training": map[string]any{"records": len(training), "tokens": tokens(training), "corpus": "AISHELL-3", "f0_source": "utautts_world_harvest", "alignment": "PaddleSpeech MFA2 with tone", "speakers": counts, "speaker_splits": map[string]any{"train": speakers[:len(speakers)-1], "validation": []string{heldOut}}, "ridge_lambda": lambda, "hyperparameter_selected_on_validation": true, "evaluation_is_unbiased_test": false, "skipped": skipped, "validation": metrics, "training_fit": evaluate(training, weights)}}, nil
}
func underOut(path string) bool {
	root, _ := filepath.Abs("out")
	target, _ := filepath.Abs(path)
	rel, err := filepath.Rel(root, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
func main() {
	observations := flag.String("observations", "", "pre-extracted AISHELL-3 observation JSONL; skips Parquet collection")
	parquetPath := flag.String("parquet", "data/aishell3/train-00000-of-00045.parquet", "AISHELL-3 Parquet shard")
	alignments := flag.String("alignments", "data/aishell3/aishell3_alignment_tone", "PaddleSpeech TextGrid directory")
	worldEngine := flag.String("world-engine", "runtime/utautts-world-engine.dll", "WORLD engine DLL")
	limit := flag.Int("limit-per-speaker", 250, "accepted utterances per speaker")
	workers := flag.Int("workers", 4, "concurrent WORLD F0 extractions")
	cache := flag.String("f0-cache", "out/tone-intonation-zh-v1/f0-cache", "WORLD F0 cache under out/")
	observationsOut := flag.String("observations-out", "", "optional collected observation JSONL under out/")
	collectOnly := flag.Bool("collect-only", false, "write observations without fitting a model")
	out := flag.String("out", "out/tone-intonation-zh-v1/tone-intonation-zh-v1.json", "new model JSON under out/")
	flag.Parse()
	if !underOut(*out) || !underOut(*cache) || (*observationsOut != "" && !underOut(*observationsOut)) {
		fmt.Fprintln(os.Stderr, "outputs and cache must be under out/")
		os.Exit(2)
	}
	if *limit < 1 || *workers < 1 || (*collectOnly && *observationsOut == "") {
		fmt.Fprintln(os.Stderr, "positive limit-per-speaker and workers, and --observations-out for collect-only, are required")
		os.Exit(2)
	}
	var rows []row
	var skipped map[string]int
	var e error
	if *observations != "" {
		rows, e = readRows(*observations)
	} else {
		rows, _, skipped, e = collect(*parquetPath, *alignments, *worldEngine, *cache, *limit, *workers)
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	if *observationsOut != "" {
		if e = writeRows(*observationsOut, rows); e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
	}
	if *collectOnly {
		fmt.Printf("collected %d utterances; skipped %v\n", len(rows), skipped)
		return
	}
	model, e := trainWithSkipped(rows, skipped)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	b, e := json.MarshalIndent(model, "", "  ")
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	if e = os.MkdirAll(filepath.Dir(*out), 0755); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	f, e := toolutil.CreateExclusive(*out)
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
	fmt.Printf("wrote %s\n", *out)
}
