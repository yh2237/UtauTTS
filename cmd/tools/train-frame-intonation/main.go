// train-frame-intonation trains a frame TCN and exports UtauTTS prosody JSON.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/yh2237/gograd/autograd"
	"github.com/yh2237/gograd/cuda"
	"github.com/yh2237/gograd/tensor"
)

type config struct {
	Dataset, Output, Cache, Language, ID, Name, Corpus, License, Device, F0Source, WorldEngine string
	Notices                                                                                    []string
	Epochs, Hidden, Batch, Seed, Limit                                                         int
	LR, Delta, Smooth, Frame, Low, High                                                        float64
	RenderStrength, RenderSmoothing, RenderP99, RenderMax                                      float64
	Dilations                                                                                  []int
	Holdout, FeaturesOnly, IndexOnly                                                           bool
}
type notices []string

func (n *notices) String() string     { return strings.Join(*n, ",") }
func (n *notices) Set(v string) error { *n = append(*n, v); return nil }
func parseFlags() config {
	var c config
	var nn notices
	var dil string
	flag.StringVar(&c.Dataset, "dataset", "", "version-1 JSONL")
	flag.StringVar(&c.Output, "out", filepath.Join("out", "frame-intonation-go", fmt.Sprintf("model-%d.json", time.Now().UnixNano())), "new model JSON under out/")
	flag.StringVar(&c.Cache, "f0-cache", "", "F0 .npy cache under out/")
	flag.StringVar(&c.F0Source, "f0-source", "auto", "internal or world")
	flag.StringVar(&c.WorldEngine, "world-engine", "runtime/utautts-world-engine.dll", "WORLD engine DLL for English Harvest")
	flag.StringVar(&c.Language, "language", "ja", "ja or en")
	flag.StringVar(&c.ID, "model-id", "", "model ID")
	flag.StringVar(&c.Name, "display-name", "", "display name")
	flag.StringVar(&c.Corpus, "training-corpus", "", "training corpus provenance")
	flag.StringVar(&c.License, "model-license", "MIT License", "model license")
	flag.StringVar(&c.Device, "device", "auto", "auto, cuda or cpu")
	flag.Var(&nn, "license-notice", "provenance notice, repeatable")
	flag.IntVar(&c.Epochs, "epochs", 24, "epochs")
	flag.IntVar(&c.Hidden, "hidden", 32, "hidden width")
	flag.IntVar(&c.Batch, "batch-size", 32, "utterances per batch")
	flag.IntVar(&c.Seed, "seed", 1, "random seed")
	flag.IntVar(&c.Limit, "limit", 0, "first N records, 0 for all")
	flag.Float64Var(&c.LR, "learning-rate", .002, "AdamW learning rate")
	flag.Float64Var(&c.Delta, "delta-weight", .35, "adjacent frame loss weight")
	flag.Float64Var(&c.Smooth, "target-smooth-ms", 40, "target log F0 smoothing width")
	flag.Float64Var(&c.Frame, "frame-ms", 10, "frame period")
	flag.Float64Var(&c.Low, "low-cents", -250, "low bound")
	flag.Float64Var(&c.High, "high-cents", 250, "high bound")
	flag.Float64Var(&c.RenderStrength, "render-strength", .32, "runtime contour strength")
	flag.Float64Var(&c.RenderSmoothing, "render-smoothing-ms", 20, "runtime contour Gaussian smoothing")
	flag.Float64Var(&c.RenderP99, "render-p99-cents", 75, "runtime contour p99 limit")
	flag.Float64Var(&c.RenderMax, "render-max-cents", 90, "runtime contour maximum")
	flag.StringVar(&dil, "dilations", "1,2,4,8,16,32", "comma-separated TCN dilations")
	flag.BoolVar(&c.Holdout, "holdout-test", true, "reserve hash(id) modulo 10 == 1 for test")
	flag.BoolVar(&c.FeaturesOnly, "features-only", false, "prepare data and print counts without training")
	flag.BoolVar(&c.IndexOnly, "index-only", false, "build the sorted feature dictionary without F0 extraction")
	flag.Parse()
	visited := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { visited[f.Name] = true })
	if c.Language == "en" {
		if !visited["epochs"] {
			c.Epochs = 40
		}
		if !visited["batch-size"] {
			c.Batch = 16
		}
		if !visited["dilations"] {
			dil = "1,2,4,8"
		}
		if !visited["model-license"] {
			c.License = "CC BY 4.0 (model weights)"
		}
		if !visited["low-cents"] {
			c.Low = -400
		}
		if !visited["high-cents"] {
			c.High = 400
		}
		if !visited["render-strength"] {
			c.RenderStrength = .65
		}
		if !visited["render-p99-cents"] {
			c.RenderP99 = 200
		}
		if !visited["render-max-cents"] {
			c.RenderMax = 250
		}
	}
	c.Notices = nn
	if c.F0Source == "auto" {
		if c.Language == "en" {
			c.F0Source = "world"
		} else {
			c.F0Source = "internal"
		}
	}
	if c.Cache == "" {
		if c.Language == "ja" {
			c.Cache = "out/mfa-align-20261002/f0-internal"
		} else {
			c.Cache = "out/frame-intonation-go/english-f0-cache"
		}
	}
	for _, s := range strings.Split(dil, ",") {
		v, e := strconv.Atoi(strings.TrimSpace(s))
		if e != nil || v < 1 {
			fatal("invalid dilation %q", s)
		}
		c.Dilations = append(c.Dilations, v)
	}
	return c
}
func fatal(format string, args ...any) { fmt.Fprintf(os.Stderr, format+"\n", args...); os.Exit(1) }
func main() {
	c := parseFlags()
	if c.Dataset == "" || c.Corpus == "" || len(c.Notices) == 0 {
		fatal("--dataset, --training-corpus and --license-notice are required")
	}
	if c.Language != "ja" && c.Language != "en" {
		fatal("language must be ja or en")
	}
	if c.RenderStrength <= 0 || c.RenderStrength > 1 || c.RenderSmoothing < 0 || c.RenderP99 <= 0 || c.RenderMax < c.RenderP99 {
		fatal("invalid renderer settings")
	}
	if c.F0Source != "internal" && c.F0Source != "world" {
		fatal("f0-source must be internal or world")
	}
	if !strings.HasPrefix(filepath.Clean(c.Cache), "out"+string(filepath.Separator)) {
		fatal("F0 cache must be under out/")
	}
	if c.Batch < 1 || c.Hidden < 1 || c.Epochs < 0 || c.Frame <= 0 || c.Low >= c.High {
		fatal("invalid training configuration")
	}
	if !strings.HasPrefix(filepath.Clean(c.Output), "out"+string(filepath.Separator)) {
		fatal("output must be under out/")
	}
	if _, e := os.Stat(c.Output); e == nil {
		fatal("refusing to overwrite %s", c.Output)
	}
	source, e := os.ReadFile(c.Dataset)
	if e != nil {
		fatal("dataset: %v", e)
	}
	sha := fmt.Sprintf("%x", sha256.Sum256(source))
	rows, e := loadRecords(c.Dataset)
	if e != nil {
		fatal("dataset: %v", e)
	}
	if c.Limit > 0 && c.Limit < len(rows) {
		rows = rows[:c.Limit]
	}
	for _, r := range rows {
		language := r.Language
		if language == "" {
			language = "ja"
		}
		if language != c.Language {
			fatal("language mismatch: %s", r.ID)
		}
		if c.Language == "ja" {
			for _, t := range r.Tokens {
				if !t.Pause && t.AccentLength == 0 {
					fatal("%s lacks Open JTalk annotations; prepare the JSONL with Open JTalk first", r.ID)
				}
			}
		}
	}
	if c.Language == "en" {
		if e := validateEnglishSplits(rows); e != nil {
			fatal("dataset: %v", e)
		}
	}
	trainRows, validRows, testRows := splitRecords(rows, c.Holdout)
	if len(trainRows) == 0 || len(validRows) == 0 {
		fatal("empty train or validation split")
	}
	names := featureNames(trainRows, c.Frame)
	if c.IndexOnly {
		fmt.Printf("dataset_sha256=%s features=%d train=%d validation=%d test=%d\n", sha, len(names), len(trainRows), len(validRows), len(testRows))
		return
	}
	index := map[string]int{}
	for i, name := range names {
		index[name] = i
	}
	start := time.Now()
	prepare := func(rs []record) []example {
		items := make([]example, 0, len(rs))
		for i, r := range rs {
			ex, e := prepareRecordSource(r, index, c.Frame, c.Smooth, c.Low, c.High, c.Cache, c.F0Source, c.WorldEngine)
			if e != nil {
				fatal("prepare: %v", e)
			}
			items = append(items, ex)
			if (i+1)%100 == 0 {
				fmt.Printf("prepared %d/%d\n", i+1, len(rs))
			}
		}
		return items
	}
	train, valid, test := prepare(trainRows), prepare(validRows), prepare(testRows)
	fmt.Printf("dataset_sha256=%s features=%d train=%d validation=%d test=%d preparation=%s\n", sha, len(names), len(train), len(valid), len(test), time.Since(start).Round(time.Millisecond))
	if c.FeaturesOnly {
		return
	}
	device := tensor.CPU
	if c.Device == "cuda" || (c.Device == "auto" && cuda.Available()) {
		device = tensor.CUDA
	}
	if c.Device == "cuda" && !cuda.Available() {
		fatal("CUDA unavailable")
	}
	if c.Device != "auto" && c.Device != "cuda" && c.Device != "cpu" {
		fatal("invalid device %q", c.Device)
	}
	c.Device = string(device)
	if device == tensor.CUDA {
		ctx, e := autograd.NewCUDAContext()
		if e != nil {
			fatal("CUDA context: %v", e)
		}
		defer ctx.Close()
	}
	model, e := newTCN(len(names), c.Hidden, c.Dilations, device, int64(c.Seed))
	if e != nil {
		fatal("model: %v", e)
	}
	params := model.Module.NamedParameters()
	opt := autograd.NewAdamW(params, float32(c.LR), 1e-5)
	rng := rand.New(rand.NewSource(int64(c.Seed)))
	best := math.Inf(1)
	bestEpoch := 0
	var bestState map[string][]float32
	history := []map[string]any{}
	trainingStart := time.Now()
	for epoch := 0; epoch < c.Epochs; epoch++ {
		order := rng.Perm(len(train))
		lossSum := 0.0
		steps := 0
		for offset := 0; offset < len(order); offset += c.Batch {
			end := min(len(order), offset+c.Batch)
			x, y, mask, _, e := batchExamples(train, order[offset:end], len(names), device)
			if e != nil {
				fatal("batch: %v", e)
			}
			opt.ZeroGrad()
			pred := model.forward(x)
			var owned []*autograd.Tensor
			scale := math.Max(1, math.Max(math.Abs(c.Low), math.Abs(c.High)))
			loss := sequenceLoss(pred, y, mask, c.Delta, float32(c.Low/scale), float32(c.High/scale), device, &owned)
			v, e := loss.ToHost()
			if e != nil {
				fatal("loss: %v", e)
			}
			if e = loss.Backward(); e != nil {
				fatal("backward: %v", e)
			}
			autograd.ClipGradNorm(params, 1)
			opt.Step()
			lossSum += float64(v[0])
			steps++
			loss.ReleaseGraph()
			for _, item := range owned {
				item.Close()
			}
			x.Close()
			y.Close()
		}
		raw, rendered, e := evaluate(model, valid, len(names), c.Batch, c)
		if e != nil {
			fatal("evaluation: %v", e)
		}
		history = append(history, map[string]any{"epoch": epoch + 1, "validation_mae_cents": raw, "rendered_mae_cents": rendered})
		if rendered < best {
			best = rendered
			bestEpoch = epoch + 1
			bestState = model.Module.StateDict()
		}
		fmt.Printf("epoch %02d/%d loss=%.6f validation=%.3f rendered=%.3f best=%d elapsed=%s\n", epoch+1, c.Epochs, lossSum/float64(max(1, steps)), raw, rendered, bestEpoch, time.Since(trainingStart).Round(time.Second))
	}
	if bestState == nil {
		fatal("training produced no checkpoint")
	}
	if e = model.Module.LoadStateDict(bestState); e != nil {
		fatal("restore: %v", e)
	}
	validation, _, e := evaluate(model, valid, len(names), c.Batch, c)
	if e != nil {
		fatal("validation: %v", e)
	}
	testRaw, testRendered := 0.0, 0.0
	if len(test) > 0 {
		testRaw, testRendered, e = evaluate(model, test, len(names), c.Batch, c)
		if e != nil {
			fatal("test: %v", e)
		}
	}
	payload := export(model, names, c, sha, trainRows, validRows, testRows, train, valid, validation, best, bestEpoch, testRaw, testRendered, history)
	data, e := json.MarshalIndent(payload, "", "  ")
	if e != nil {
		fatal("export: %v", e)
	}
	if e = os.MkdirAll(filepath.Dir(c.Output), 0755); e != nil {
		fatal("output directory: %v", e)
	}
	if e = os.WriteFile(c.Output, append(data, '\n'), 0644); e != nil {
		fatal("write: %v", e)
	}
	fmt.Printf("wrote %s total=%s\n", c.Output, time.Since(start).Round(time.Second))
}
