package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/yh2237/gograd/autograd"
	"github.com/yh2237/gograd/tensor"
	"utautts/cmd/tools/internal/toolutil"
)

const trainingFormat = "utautts-train-speech-timing-1"
const continuousFeatures = 4

// キャッシュの同一性、分割、乱数状態、最良モデルを含む再開情報。
type trainingState struct {
	CacheSHA256                            string
	Steps, Valid, Batch, Window, EvalEvery int
	Seed                                   int64
	TrainingCorpus                         string
	Notices                                []string
	Order                                  []int
	Sampler                                autograd.WindowSamplerState
	Best                                   float64
	BestStep                               int
}

func train(ctx context.Context, c trainingConfig) error {
	if c.Checkpoint == "" {
		c.Checkpoint = c.Out + ".training.safetensors"
	}
	if err := validatePaths(c); err != nil {
		return err
	}
	if !c.FeaturesOnly {
		if c.Steps < 2 || c.Batch < 1 || c.Window < 1 || c.EvalEvery < 1 || c.CheckpointEvery < 1 || c.StopAfter < 0 || c.StopAfter > c.Steps {
			return fmt.Errorf("invalid training steps, batch, window or interval")
		}
		if err := requireNewPath(c.Out); err != nil {
			return err
		}
		if err := requireNewPath(c.Fixture); err != nil {
			return err
		}
		if c.Resume == "" || !samePath(c.Checkpoint, c.Resume) {
			if err := requireNewPath(c.Checkpoint); err != nil {
				return err
			}
		}
	}
	if c.FeaturesJSON != "" {
		if err := requireNewPath(c.FeaturesJSON); err != nil {
			return err
		}
	}
	if _, err := os.Stat(c.Cache); os.IsNotExist(err) && c.Corpus == "" && (c.Dataset == "" || c.Alignments == "") {
		return fmt.Errorf("--corpus or --dataset and --alignments are required to build a new feature cache")
	}
	items, phones, err := loadOrBuildFeatures(c.Cache, c.Dataset, c.Corpus, c.Alignments, c.WorldEngine)
	if err != nil {
		return fmt.Errorf("read features: %w", err)
	}
	vocab := newVocabulary(strings.Fields(phones))
	for _, item := range items {
		if item.Frames < 1 || item.Continuous < continuousFeatures || len(item.IDs) != item.Frames*3 || len(item.Cont) != item.Frames*item.Continuous || len(item.Target) != item.Frames*80 {
			return fmt.Errorf("cache %s has incompatible frames/features", item.ID)
		}
	}
	if c.FeaturesJSON != "" {
		data, err := json.Marshal(items[:min(3, len(items))])
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(c.FeaturesJSON), 0755); err != nil {
			return err
		}
		file, err := toolutil.CreateExclusive(c.FeaturesJSON)
		if err != nil {
			return err
		}
		_, err = file.Write(data)
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	if c.FeaturesOnly {
		fmt.Printf("utterances=%d frames=%d cache=%s\n", len(items), totalFrames(items), c.Cache)
		return nil
	}
	if c.Valid < 1 || c.Valid >= len(items) {
		return fmt.Errorf("invalid validation count")
	}
	hash, err := cacheDigest(c.Cache)
	if err != nil {
		return err
	}
	state := trainingState{CacheSHA256: hash, Steps: c.Steps, Valid: c.Valid, Batch: c.Batch, Window: c.Window,
		EvalEvery: c.EvalEvery, Seed: c.Seed, TrainingCorpus: c.TrainingCorpus, Notices: slices.Clone(c.Notices), Best: math.Inf(1), BestStep: -1}
	if c.Resume != "" {
		metadata, err := autograd.TrainingCheckpointMetadata(c.Resume)
		if err != nil {
			return err
		}
		if metadata["tool"] != trainingFormat {
			return fmt.Errorf("checkpoint is not from this training tool")
		}
		var saved trainingState
		if err := json.Unmarshal([]byte(metadata["trainer"]), &saved); err != nil {
			return err
		}
		if saved.CacheSHA256 != hash || saved.Steps != c.Steps || saved.Valid != c.Valid || saved.Batch != c.Batch || saved.Window != c.Window || saved.EvalEvery != c.EvalEvery || saved.Seed != c.Seed || saved.TrainingCorpus != c.TrainingCorpus || !slices.Equal(saved.Notices, c.Notices) || saved.BestStep < 0 || saved.BestStep >= c.Steps || saved.Best < 0 || math.IsNaN(saved.Best) || math.IsInf(saved.Best, 0) {
			return fmt.Errorf("resume checkpoint does not match cache or training configuration")
		}
		state = saved
	} else {
		state.Order = rand.New(rand.NewSource(c.Seed)).Perm(len(items))
		fmt.Println("split=Go RNG")
	}
	if err := validateOrder(state.Order, len(items)); err != nil {
		return err
	}
	validation, training := make([]utterance, c.Valid), make([]utterance, len(items)-c.Valid)
	for i, index := range state.Order {
		if i < c.Valid {
			validation[i] = items[index]
		} else {
			training[i-c.Valid] = items[index]
		}
	}
	frames := make([]int, len(training))
	for i, item := range training {
		frames[i] = item.Frames
	}
	sampler, err := autograd.NewWindowSampler(frames, c.Window, c.Batch, 0, uint64(c.Seed))
	if err != nil {
		return err
	}
	if c.Resume != "" {
		if err := sampler.LoadState(state.Sampler); err != nil {
			return err
		}
	}
	device, err := toolutil.ResolveDevice(c.Device)
	if err != nil {
		return err
	}
	if device == tensor.CUDA {
		cudaContext, err := autograd.NewCUDAContext()
		if err != nil {
			return err
		}
		defer cudaContext.Close()
	}
	model, err := autograd.NewSpeechTimingWithPhones(len(vocab.names), continuousFeatures, device, c.Seed)
	if err != nil {
		return err
	}
	defer closeModule(&model.Module)
	bestModel, err := autograd.NewSpeechTimingWithPhones(len(vocab.names), continuousFeatures, tensor.CPU, c.Seed)
	if err != nil {
		return err
	}
	defer closeModule(&bestModel.Module)
	combined := &autograd.Module{Children: []autograd.NamedModule{{Name: "current", Module: &model.Module}, {Name: "best", Module: &bestModel.Module}}}
	opt := autograd.NewAdamW(model.Parameters(), .002, .0001)
	defer opt.Close()
	schedule := autograd.NewOneCycle(.002, c.Steps, .1)
	opt.LR = float32(schedule.LR())
	limit := c.Steps
	if c.StopAfter > 0 {
		limit = c.StopAfter
	}
	if c.Resume != "" {
		if _, err := autograd.LoadTrainingCheckpoint(c.Resume, combined, opt, schedule); err != nil {
			return err
		}
		if schedule.Total != c.Steps || state.BestStep >= opt.StepCount || limit < opt.StepCount {
			return fmt.Errorf("inconsistent resume step or --stop-after")
		}
		if err := os.MkdirAll(filepath.Dir(c.Out), 0700); err != nil {
			return err
		}
		if err := bestModel.Module.SaveSafeTensorsMetadata(c.Out, checkpointMetadata(state.Best, state.BestStep, c.TrainingCorpus, c.Notices, phones, modelIDForLanguage(c.Language))); err != nil {
			return err
		}
		fmt.Printf("resumed step=%d best=%.6f@%d\n", opt.StepCount, state.Best, state.BestStep)
	}
	model.Train(true)
	started := time.Now()
	save := func() error {
		state.Sampler = sampler.State()
		data, err := json.Marshal(state)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(c.Checkpoint), 0700); err != nil {
			return err
		}
		return autograd.SaveTrainingCheckpoint(c.Checkpoint, combined, opt, schedule, map[string]string{"tool": trainingFormat, "trainer": string(data)})
	}
	fmt.Printf("device=%s utterances=%d train=%d valid=%d frames=%d steps=%d batch=%d window=%d output=%s\n", device, len(items), len(training), len(validation), totalFrames(items), c.Steps, c.Batch, c.Window, c.Out)
	for step := opt.StepCount; step < limit; step++ {
		if ctx.Err() != nil {
			if opt.StepCount == 0 {
				return ctx.Err()
			}
			fmt.Println("interrupted: saving at completed update", opt.StepCount)
			break
		}
		ids, cont, target := sampleBatch(training, sampler, c.Batch, c.Window)
		loss, err := update(model, opt, schedule, ids, cont, target, c.Batch, c.Window, device, uint32(c.Seed)+uint32(step)*8)
		if err != nil {
			return err
		}
		if step%c.EvalEvery == 0 || step == c.Steps-1 {
			score, err := evaluate(model, validation, device, vocab.silence)
			if err != nil {
				return err
			}
			if math.IsNaN(score) || math.IsInf(score, 0) {
				return fmt.Errorf("non-finite validation score")
			}
			if score < state.Best {
				state.Best, state.BestStep = score, step
				if err := bestModel.Module.LoadStateDict(model.Module.StateDict()); err != nil {
					return err
				}
				if err := os.MkdirAll(filepath.Dir(c.Out), 0700); err != nil {
					return err
				}
				if err := bestModel.Module.SaveSafeTensorsMetadata(c.Out, checkpointMetadata(score, step, c.TrainingCorpus, c.Notices, phones, modelIDForLanguage(c.Language))); err != nil {
					return err
				}
			}
			fmt.Printf("step=%d loss=%.5f valid=%.5f best=%.5f@%d elapsed=%s\n", step, loss, score, state.Best, state.BestStep, time.Since(started).Round(time.Second))
		}
		if opt.StepCount%c.CheckpointEvery == 0 && opt.StepCount != limit {
			if err := save(); err != nil {
				return err
			}
		}
	}
	// fixture作成のための最良重みの復元より先に、最新の学習状態を保存する。
	if err := save(); err != nil {
		return err
	}
	if err := model.Module.LoadStateDict(bestModel.Module.StateDict()); err != nil {
		return err
	}
	if err := writeFixture(c.Fixture, model, device, len(vocab.names)); err != nil {
		return err
	}
	fmt.Printf("finished completed=%d planned=%d best_valid_l1=%.6f best_step=%d wall=%s checkpoint=%s\n", opt.StepCount, c.Steps, state.Best, state.BestStep, time.Since(started).Round(time.Millisecond), c.Checkpoint)
	return nil
}

func update(model *autograd.SpeechTiming, opt *autograd.AdamW, schedule *autograd.OneCycle, ids []int, cv, tv []float32, batch, window int, device tensor.Device, dropoutSeed uint32) (float32, error) {
	cont, err := autograd.New(cv, []int{batch, window, continuousFeatures}, device, false)
	if err != nil {
		return 0, err
	}
	defer cont.Close()
	target, err := autograd.New(tv, []int{batch, window, 80}, device, false)
	if err != nil {
		return 0, err
	}
	defer target.Close()
	opt.ZeroGrad()
	loss := autograd.MaskedLoss(model.Forward(ids, cont, dropoutSeed), target, false)
	defer loss.ReleaseGraph()
	values, err := loss.ToHost()
	if err != nil {
		return 0, err
	}
	if math.IsNaN(float64(values[0])) || math.IsInf(float64(values[0]), 0) {
		return 0, fmt.Errorf("non-finite training loss")
	}
	if err := loss.Backward(); err != nil {
		return 0, err
	}
	autograd.ClipGradNorm(opt.Params, 1)
	opt.Step()
	schedule.Step(opt)
	return values[0], nil
}

func sampleBatch(items []utterance, sampler *autograd.WindowSampler, batch, window int) ([]int, []float32, []float32) {
	ids, cont, target := make([]int, batch*window*3), make([]float32, batch*window*continuousFeatures), make([]float32, batch*window*80)
	for b, sampled := range sampler.Batch() {
		item := items[sampled.Index]
		for t := 0; t < window; t++ {
			dst, src := b*window+t, sampled.Start+t
			if src >= item.Frames {
				for j := 0; j < 80; j++ {
					target[dst*80+j] = float32(math.NaN())
				}
				continue
			}
			copy(ids[dst*3:dst*3+3], item.IDs[src*3:src*3+3])
			copy(cont[dst*continuousFeatures:(dst+1)*continuousFeatures], item.Cont[src*item.Continuous:src*item.Continuous+continuousFeatures])
			copy(target[dst*80:dst*80+80], item.Target[src*80:src*80+80])
		}
	}
	return ids, cont, target
}

func evaluate(model *autograd.SpeechTiming, items []utterance, device tensor.Device, silence int) (float64, error) {
	wasTraining := model.Module.Training
	model.Train(false)
	defer model.Train(wasTraining)
	var scores float64
	for _, item := range items {
		cv := make([]float32, item.Frames*continuousFeatures)
		for t := 0; t < item.Frames; t++ {
			copy(cv[t*continuousFeatures:(t+1)*continuousFeatures], item.Cont[t*item.Continuous:t*item.Continuous+continuousFeatures])
		}
		cont, err := autograd.New(cv, []int{1, item.Frames, continuousFeatures}, device, false)
		if err != nil {
			return 0, err
		}
		var pred *autograd.Tensor
		autograd.NoGrad(func() { pred = model.Forward(item.IDs, cont, 0) })
		values, err := pred.ToHost()
		pred.ReleaseGraph()
		cont.Close()
		if err != nil {
			return 0, err
		}
		var total float64
		count := 0
		for t := 0; t < item.Frames; t++ {
			if item.IDs[t*3] == silence {
				continue
			}
			for j := 0; j < 80; j++ {
				total += math.Abs(float64(values[t*80+j] - item.Target[t*80+j]))
				count++
			}
		}
		if count > 0 {
			scores += total / float64(count)
		}
	}
	return scores / float64(len(items)), nil
}

func requireNewPath(path string) error {
	if path == "" {
		return fmt.Errorf("output path is required")
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("refusing to overwrite %s", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func samePath(a, b string) bool {
	a, errA := filepath.Abs(a)
	b, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func validatePaths(c trainingConfig) error {
	outputs := []string{c.FeaturesJSON}
	if !c.FeaturesOnly {
		outputs = append(outputs, c.Out, c.Fixture, c.Checkpoint)
	}
	for i, output := range outputs {
		if output == "" {
			continue
		}
		for _, input := range []string{c.Cache, c.Dataset, c.Corpus} {
			if input == "" {
				continue
			}
			if samePath(output, input) {
				return fmt.Errorf("output and input paths must differ: %s", output)
			}
		}
		if c.Resume != "" && output != c.Checkpoint && samePath(output, c.Resume) {
			return fmt.Errorf("output must not replace the resume checkpoint: %s", output)
		}
		for _, previous := range outputs[:i] {
			if previous != "" && samePath(output, previous) {
				return fmt.Errorf("output paths must differ: %s", output)
			}
		}
	}
	return nil
}

func cacheDigest(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func validateOrder(order []int, count int) error {
	if len(order) != count {
		return fmt.Errorf("resume split length mismatch")
	}
	sorted := slices.Clone(order)
	slices.Sort(sorted)
	for i, n := range sorted {
		if n != i {
			return fmt.Errorf("resume split is not a permutation")
		}
	}
	return nil
}

func closeModule(module *autograd.Module) {
	for _, p := range module.NamedParameters() {
		p.Value.Close()
	}
}
