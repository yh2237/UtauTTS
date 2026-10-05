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
	"time"

	"github.com/yh2237/gograd/autograd"
	"github.com/yh2237/gograd/tensor"
	"utautts/cmd/tools/internal/toolutil"
	"utautts/internal/openjtalk"
)

type config struct {
	BaseModel, Out, ModelID, DisplayName, Description, Device, Helper, Dictionary                       string
	Hidden, Epochs, Batch, MinEntries, Seed                                                             int
	LearningRate, TargetScale, MaximumResidual, SmoothingMS, ZeroPrior, DeltaWeight, ValidationFraction float64
	Sessions                                                                                            []string
}

func defaults() config {
	return config{Hidden: 24, Epochs: 500, Batch: 8, LearningRate: .003, TargetScale: 120, MaximumResidual: 180, SmoothingMS: 20, ZeroPrior: .002, DeltaWeight: .2, ValidationFraction: .2, MinEntries: 8, Seed: 17, Device: "auto"}
}
func parse() config {
	c := defaults()
	flag.StringVar(&c.BaseModel, "base-model", "", "shipped frame model JSON")
	flag.StringVar(&c.Out, "out", "", "new model JSON under out/")
	flag.StringVar(&c.ModelID, "model-id", "", "model identifier")
	flag.StringVar(&c.DisplayName, "display-name", "", "display name")
	flag.StringVar(&c.Description, "description", "", "model description")
	flag.StringVar(&c.Device, "device", c.Device, "auto, cpu, cuda")
	flag.StringVar(&c.Helper, "openjtalk-helper", "", "native Open JTalk helper")
	flag.StringVar(&c.Dictionary, "openjtalk-dictionary", "", "Open JTalk dictionary")
	flag.IntVar(&c.Hidden, "hidden", c.Hidden, "hidden width")
	flag.IntVar(&c.Epochs, "epochs", c.Epochs, "epochs")
	flag.IntVar(&c.Batch, "batch-size", c.Batch, "batch size")
	flag.IntVar(&c.MinEntries, "min-entries", c.MinEntries, "minimum accepted Lab entries")
	flag.IntVar(&c.Seed, "seed", c.Seed, "seed")
	flag.Float64Var(&c.LearningRate, "learning-rate", c.LearningRate, "AdamW rate")
	flag.Float64Var(&c.TargetScale, "target-scale", c.TargetScale, "target cents scale")
	flag.Float64Var(&c.MaximumResidual, "maximum-residual", c.MaximumResidual, "target clipping in cents")
	flag.Float64Var(&c.SmoothingMS, "smoothing-ms", c.SmoothingMS, "render smoothing")
	flag.Float64Var(&c.ZeroPrior, "zero-prior", c.ZeroPrior, "squared zero penalty")
	flag.Float64Var(&c.DeltaWeight, "delta-weight", c.DeltaWeight, "neighbor delta loss")
	flag.Float64Var(&c.ValidationFraction, "validation-fraction", c.ValidationFraction, "held-out Lab entries")
	flag.Parse()
	c.Sessions = flag.Args()
	return c
}
func train(c config) (map[string]any, error) {
	if c.BaseModel == "" || c.Out == "" || len(c.Sessions) == 0 {
		return nil, fmt.Errorf("sessions, --base-model and --out are required")
	}
	if e := outputPath(c.Out); e != nil {
		return nil, e
	}
	if c.Hidden <= 0 || c.Epochs <= 0 || c.Batch <= 0 || c.TargetScale <= 0 || c.MaximumResidual <= 0 || c.ValidationFraction < 0 || c.ValidationFraction >= 1 {
		return nil, fmt.Errorf("invalid training settings")
	}
	raw, e := os.ReadFile(c.BaseModel)
	if e != nil {
		return nil, e
	}
	var base map[string]any
	if e = json.Unmarshal(raw, &base); e != nil {
		return nil, e
	}
	if asString(base["id"]) == "" || asMap(base["frame_pitch"]) == nil {
		return nil, fmt.Errorf("base model must have id and frame_pitch")
	}
	rows, accepted, skipped, e := loadPhrases(c.Sessions, c.MaximumResidual, openjtalk.Config{HelperPath: c.Helper, DictionaryPath: c.Dictionary})
	if e != nil {
		return nil, e
	}
	for _, reason := range skipped {
		fmt.Fprintln(os.Stderr, "skip:", reason)
	}
	if accepted < c.MinEntries {
		return nil, fmt.Errorf("only %d accepted entries; need at least %d", accepted, c.MinEntries)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no usable sounding phrases")
	}
	names := featureNames(rows)
	idsMap := map[string]bool{}
	for _, r := range rows {
		idsMap[r.ID] = true
	}
	ids := make([]string, 0, len(idsMap))
	for id := range idsMap {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	rng := rand.New(rand.NewSource(int64(c.Seed)))
	rng.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	n := 0
	if len(ids) >= 5 {
		n = int(math.RoundToEven(float64(len(ids)) * c.ValidationFraction))
	}
	validationIDs := map[string]bool{}
	for _, id := range ids[:n] {
		validationIDs[id] = true
	}
	var training, validation []phrase
	for _, r := range rows {
		if validationIDs[r.ID] {
			validation = append(validation, r)
		} else {
			training = append(training, r)
		}
	}
	if len(training) == 0 {
		training = rows
	}
	if len(validation) == 0 {
		validation = training
	}
	device, e := toolutil.ResolveDevice(c.Device)
	if e != nil {
		return nil, e
	}
	if device == tensor.CUDA {
		ctx, e := autograd.NewCUDAContext()
		if e != nil {
			return nil, e
		}
		defer ctx.Close()
	}
	model, e := newTCN(len(names), c.Hidden, device, int64(c.Seed))
	if e != nil {
		return nil, e
	}
	params := model.Module.NamedParameters()
	opt := autograd.NewAdamW(params, float32(c.LearningRate), 1e-4)
	best := math.Inf(1)
	var bestState map[string][]float32
	start := time.Now()
	for epoch := 0; epoch < c.Epochs; epoch++ {
		order := rng.Perm(len(training))
		for offset := 0; offset < len(order); offset += c.Batch {
			last := min(len(order), offset+c.Batch)
			items := make([]phrase, last-offset)
			for i, id := range order[offset:last] {
				items[i] = training[id]
			}
			x, y, mask, length, e := batch(items, names, c.TargetScale, device)
			if e != nil {
				return nil, e
			}
			opt.ZeroGrad()
			pred := model.forward(x)
			var owned []*autograd.Tensor
			loss := residualLoss(pred, y, mask, length, c.DeltaWeight, c.ZeroPrior, device, &owned)
			if e = loss.Backward(); e != nil {
				return nil, e
			}
			autograd.ClipGradNorm(params, 1)
			opt.Step()
			loss.ReleaseGraph()
			for _, v := range owned {
				v.Close()
			}
			x.Close()
			y.Close()
		}
		now, e := score(model, validation, names, c.TargetScale)
		if e != nil {
			return nil, e
		}
		if now < best {
			best = now
			bestState = model.Module.StateDict()
		}
		if (epoch+1)%50 == 0 || epoch+1 == c.Epochs {
			fmt.Printf("epoch %d/%d: mae=%.2f cents elapsed=%s\n", epoch+1, c.Epochs, now, time.Since(start).Round(time.Second))
		}
	}
	if e = model.Module.LoadStateDict(bestState); e != nil {
		return nil, e
	}
	mae, e := score(model, validation, names, c.TargetScale)
	if e != nil {
		return nil, e
	}
	stem := filepath.Base(c.Out)
	stem = stem[:len(stem)-len(filepath.Ext(stem))]
	id := c.ModelID
	if id == "" {
		id = stem
	}
	name := c.DisplayName
	if name == "" {
		name = stem
	}
	description := c.Description
	if description == "" {
		description = "Manual intonation residual trained in UtauTTS Intonation Lab"
	}
	sum := sha256.Sum256(raw)
	baseID := asString(base["id"])
	delete(base, "mora_duration")
	delete(base, "english_intonation")
	base["id"] = id
	base["display_name"] = name
	base["description"] = description
	base["version"] = 11
	base["feature_version"] = 2
	base["mode"] = "intonation_frame_manual_residual"
	base["outputs"] = map[string]any{"frame_pitch": true, "mora_pitch_residual": true}
	base["duration_weights"] = map[string]any{}
	base["mora_pitch_residual"] = exportResidual(model, names, c.Hidden, c.TargetScale)
	base["base_model"] = map[string]any{"id": baseID, "sha256": fmt.Sprintf("%x", sum)}
	base["residual_limits"] = map[string]any{"low_cents": -c.MaximumResidual, "high_cents": c.MaximumResidual, "smoothing_ms": c.SmoothingMS}
	base["metrics"] = map[string]any{"records": accepted, "tokens": len(rows), "pitch_mae_cents": mae}
	base["training"] = map[string]any{"records": accepted, "tokens": len(rows), "epochs": c.Epochs, "learning_rate": c.LearningRate, "seed": c.Seed}
	return base, nil
}
func writeModel(path string, model map[string]any) error {
	if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		return e
	}
	b, e := json.MarshalIndent(model, "", "  ")
	if e != nil {
		return e
	}
	f, e := toolutil.CreateExclusive(path)
	if e != nil {
		return e
	}
	_, e = f.Write(append(b, '\n'))
	closeError := f.Close()
	if e != nil {
		return e
	}
	return closeError
}
func main() {
	c := parse()
	model, e := train(c)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	if e = writeModel(c.Out, model); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	fmt.Printf("wrote %s\n", c.Out)
}
