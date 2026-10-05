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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yh2237/gograd/autograd"
	"github.com/yh2237/gograd/tensor"

	"utautts/cmd/tools/internal/toolutil"
)

type config struct {
	Dataset, Output, Cache, Language, ID, Name, Corpus, License, Device, F0Source, WorldEngine string
	PredictCorpus, PredictOut, OpenJTalkHelper, OpenJTalkDictionary                            string
	Description, AudioRoot                                                                     string
	Notices                                                                                    []string
	RecommendedRenderers                                                                       []string
	Epochs, Hidden, Batch, Seed, Limit                                                         int
	LR, Delta, Smooth, Frame, Low, High                                                        float64
	RenderStrength, RenderSmoothing, RenderP99, RenderMax                                      float64
	Dilations                                                                                  []int
	F0Method                                                                                   int
	Holdout, FeaturesOnly, IndexOnly, AllDataTraining, OpenJTalkAccent                         bool
}
type notices []string

func (n *notices) String() string     { return strings.Join(*n, ",") }
func (n *notices) Set(v string) error { *n = append(*n, v); return nil }
func parseFlags() config {
	var c config
	var nn notices
	var renderers notices
	var dil string
	var noOpenJTalk bool
	flag.StringVar(&c.Dataset, "dataset", "", "version-1 JSONL")
	flag.StringVar(&c.Output, "out", filepath.Join("out", "frame-intonation-go", fmt.Sprintf("model-%d.json", time.Now().UnixNano())), "new model JSON under out/")
	flag.StringVar(&c.Cache, "f0-cache", "", "F0 .npy cache under out/")
	flag.StringVar(&c.F0Source, "f0-source", "auto", "internal or world")
	flag.StringVar(&c.WorldEngine, "world-engine", "runtime/utautts-world-engine.dll", "WORLD engine DLL for English Harvest")
	flag.StringVar(&c.PredictCorpus, "predict-corpus", "", "JSON or JSONL cases for contour prediction")
	flag.StringVar(&c.PredictOut, "predict-out", "", "prediction JSON under out/")
	flag.StringVar(&c.OpenJTalkHelper, "openjtalk-features", "", "Open JTalk feature helper executable")
	flag.StringVar(&c.OpenJTalkDictionary, "openjtalk-dictionary", "", "Open JTalk dictionary directory")
	flag.StringVar(&c.Language, "language", "ja", "ja or en")
	flag.StringVar(&c.ID, "model-id", "", "model ID")
	flag.StringVar(&c.Name, "display-name", "", "display name")
	flag.StringVar(&c.Description, "description", "", "model description")
	flag.Var(&renderers, "recommended-renderer", "compatible renderer ID, repeatable")
	flag.StringVar(&c.AudioRoot, "audio-root", "", "root for relative audio paths")
	flag.IntVar(&c.F0Method, "f0-method", 1, "WORLD Harvest method (1)")
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
	flag.BoolVar(&c.AllDataTraining, "all-data-training", false, "include validation IDs in training; metrics become in-sample")
	flag.BoolVar(&c.OpenJTalkAccent, "openjtalk-accent", false, "reanalyze Japanese raw text with Open JTalk")
	flag.BoolVar(&noOpenJTalk, "no-openjtalk-accent", false, "use timed-token accent fields without raw-text reanalysis")
	flag.BoolVar(&c.FeaturesOnly, "features-only", false, "prepare data and print counts without training")
	flag.BoolVar(&c.IndexOnly, "index-only", false, "build the sorted feature dictionary without F0 extraction")
	flag.Parse()
	if c.Language == "ja" {
		c.OpenJTalkAccent = true
		flag.Visit(func(f *flag.Flag) {
			if f.Name == "openjtalk-accent" {
				c.OpenJTalkAccent = f.Value.String() == "true"
			}
		})
		if noOpenJTalk {
			c.OpenJTalkAccent = false
		}
	}
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
	c.RecommendedRenderers = renderers
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
	if c.Language == "en" && c.AllDataTraining {
		fatal("English explicit speaker splits do not support --all-data-training")
	}
	if (c.PredictCorpus == "") != (c.PredictOut == "") {
		fatal("--predict-corpus and --predict-out must be used together")
	}
	if c.PredictOut != "" && !toolutil.UnderOutChild(c.PredictOut) {
		fatal("prediction output must be under out/")
	}
	if c.PredictOut != "" {
		if _, e := os.Stat(c.PredictOut); e == nil {
			fatal("refusing to overwrite %s", c.PredictOut)
		}
	}
	if c.RenderStrength <= 0 || c.RenderStrength > 1 || c.RenderSmoothing < 0 || c.RenderP99 <= 0 || c.RenderMax < c.RenderP99 {
		fatal("invalid renderer settings")
	}
	if c.F0Source != "internal" && c.F0Source != "world" {
		fatal("f0-source must be internal or world")
	}
	if c.F0Method != 1 {
		fatal("--f0-method supports Harvest (1) only")
	}
	if !toolutil.UnderOutChild(c.Cache) {
		fatal("F0 cache must be under out/")
	}
	if c.Batch < 1 || c.Hidden < 1 || c.Epochs < 0 || c.Frame <= 0 || c.Low >= c.High {
		fatal("invalid training configuration")
	}
	if !toolutil.UnderOutChild(c.Output) {
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
	for i := range rows {
		rows[i].AudioPath = resolveAudioPath(rows[i].AudioPath, c.Dataset, c.AudioRoot)
	}
	for _, r := range rows {
		language := r.Language
		if language == "" {
			language = "ja"
		}
		if language != c.Language {
			fatal("language mismatch: %s", r.ID)
		}
		if c.Language == "ja" && !c.OpenJTalkAccent {
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
	trainRows, validRows, testRows := splitForTraining(rows, c.Holdout, c.AllDataTraining)
	inputTrain, inputValid, inputTest := len(trainRows), len(validRows), len(testRows)
	if c.OpenJTalkAccent {
		trainRows, validRows, testRows, e = reanalyzeSplits(trainRows, validRows, testRows, c)
		if e != nil {
			fatal("Open JTalk: %v", e)
		}
	}
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
	device, e := toolutil.ResolveDevice(c.Device)
	if e != nil {
		fatal("%v", e)
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
		logDeviceMemory(epoch + 1)
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
	if c.OpenJTalkAccent {
		training := payload["training"].(map[string]any)
		alignment := training["alignment"].(map[string]any)
		setAlignmentCounts(alignment["train"].(map[string]any), inputTrain, len(trainRows))
		setAlignmentCounts(alignment["validation"].(map[string]any), inputValid, len(validRows))
		alignment["skipped_records"] = inputTrain + inputValid - len(trainRows) - len(validRows)
		alignment["alignment_rate"] = float64(len(trainRows)+len(validRows)) / float64(max(1, inputTrain+inputValid))
		if len(testRows) > 0 {
			setAlignmentCounts(training["test_alignment"].(map[string]any), inputTest, len(testRows))
		}
	}
	if c.Language == "en" {
		metrics := payload["metrics"].(map[string]any)
		metrics["validation_flat_baseline"] = flatBaselineMetrics(valid, c)
		if len(test) > 0 {
			metrics["test_flat_baseline"] = flatBaselineMetrics(test, c)
		}
	}
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
	logDeviceMemory(0)
	if c.PredictCorpus != "" {
		prediction, e := predictCorpus(model, c.PredictCorpus, index, c)
		if e != nil {
			fatal("predict corpus: %v", e)
		}
		data, e := json.MarshalIndent(prediction, "", "  ")
		if e != nil {
			fatal("prediction JSON: %v", e)
		}
		if e = os.MkdirAll(filepath.Dir(c.PredictOut), 0755); e != nil {
			fatal("prediction directory: %v", e)
		}
		if e = os.WriteFile(c.PredictOut, append(data, '\n'), 0644); e != nil {
			fatal("prediction output: %v", e)
		}
	}
}

func splitForTraining(rows []record, holdout, allData bool) (train, valid, test []record) {
	train, valid, test = splitRecords(rows, holdout)
	if allData {
		train = append(train, valid...)
		train = append(train, test...)
		sort.Slice(train, func(i, j int) bool { return train[i].ID < train[j].ID })
		test = nil
	}
	return
}

func setAlignmentCounts(stats map[string]any, input, aligned int) {
	stats["skipped_records"] = input - aligned
	stats["alignment_rate"] = float64(aligned) / float64(max(1, input))
}
