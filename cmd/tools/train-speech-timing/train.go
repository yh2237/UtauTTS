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

// trainerModelは単一ヘッドとマルチヘッドを同じ学習ループで扱う。
type trainerModel struct {
	single *autograd.SpeechTiming
	multi  *autograd.SpeechTimingMultiHead
}

func newTrainerModel(c trainingConfig, phones int, device tensor.Device) (trainerModel, error) {
	if c.F0Head {
		m, err := autograd.NewSpeechTimingMultiHeadWithF0Dilations(phones, continuousFeatures, f0ContextWidth(), f0DilationsOrDefault(c.F0Dilations), device, c.Seed)
		if err != nil {
			return trainerModel{}, err
		}
		return trainerModel{multi: m}, nil
	}
	m, err := autograd.NewSpeechTimingWithPhones(phones, continuousFeatures, device, c.Seed)
	if err != nil {
		return trainerModel{}, err
	}
	return trainerModel{single: m}, nil
}

func (m trainerModel) parameters() []autograd.Parameter {
	if m.multi != nil {
		return m.multi.Parameters()
	}
	return m.single.Parameters()
}

func (m trainerModel) train(training bool) {
	if m.multi != nil {
		m.multi.Train(training)
		return
	}
	m.single.Train(training)
}

func (m trainerModel) module() *autograd.Module {
	if m.multi != nil {
		return &m.multi.Module
	}
	return &m.single.Module
}

func (m trainerModel) forward(ids []int, cont, f0Cont *autograd.Tensor, seed uint32) (*autograd.Tensor, *autograd.Tensor, *autograd.Tensor) {
	if m.multi != nil {
		return m.multi.Forward(ids, cont, f0Cont, seed)
	}
	return m.single.Forward(ids, cont, seed), nil, nil
}

func (m trainerModel) close() { closeModule(m.module()) }

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
		if c.LR <= 0 || math.IsNaN(c.LR) || math.IsInf(c.LR, 0) {
			return fmt.Errorf("invalid learning rate")
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
	items, phones, err := loadOrBuildFeatures(c.Cache, c.Dataset, c.Corpus, c.Alignments, c.WorldEngine, c.F0Teacher)
	if err != nil {
		return fmt.Errorf("read features: %w", err)
	}
	if f0PositionInput {
		if c.Dataset == "" {
			return fmt.Errorf("--f0-position needs --dataset")
		}
		if err := attachPositions(items, c.Dataset); err != nil {
			return fmt.Errorf("attach positions: %w", err)
		}
	}
	vocab := newVocabulary(strings.Fields(phones))
	for _, item := range items {
		if item.Frames < 1 || item.Continuous < continuousFeatures || len(item.IDs) != item.Frames*3 || len(item.Cont) != item.Frames*item.Continuous || len(item.Target) != item.Frames*80 {
			return fmt.Errorf("cache %s has incompatible frames/features", item.ID)
		}
		if len(item.F0Target) != item.Frames || len(item.EnergyTarget) != item.Frames || len(item.F0Extra) != item.Frames*f0ExtraFeatures {
			return fmt.Errorf("cache %s is missing F0, energy or accent targets; delete it to rebuild", item.ID)
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
	// planValidationは検証発話のプラン時間版。合成時と同じ一定のモーラ長でのF0の相関（f0r_plan）を測る。
	var planValidation []utterance
	if c.F0Head && c.Dataset != "" {
		if planValidation, err = planAugment(validation, c.Dataset, vocab, c.Seed+1); err != nil {
			return fmt.Errorf("plan validation: %w", err)
		}
	}
	if c.PlanAugment {
		augmented, err := planAugment(training, c.Dataset, vocab, c.Seed)
		if err != nil {
			return fmt.Errorf("plan augment: %w", err)
		}
		fmt.Printf("plan-augmented=%d\n", len(augmented))
		training = append(training, augmented...)
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
	model, err := newTrainerModel(c, len(vocab.names), device)
	if err != nil {
		return err
	}
	defer model.close()
	bestModel, err := newTrainerModel(c, len(vocab.names), tensor.CPU)
	if err != nil {
		return err
	}
	defer bestModel.close()
	combined := &autograd.Module{Children: []autograd.NamedModule{{Name: "current", Module: model.module()}, {Name: "best", Module: bestModel.module()}}}
	opt := autograd.NewAdamW(model.parameters(), float32(c.LR), .0001)
	defer opt.Close()
	schedule := autograd.NewOneCycle(c.LR, c.Steps, .1)
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
		if err := bestModel.module().SaveSafeTensorsMetadata(c.Out, checkpointMetadata(state.Best, state.BestStep, c.TrainingCorpus, c.Notices, phones, modelIDForLanguage(c.Language), c.F0Head, c.F0Teacher != "", c.F0Dilations)); err != nil {
			return err
		}
		fmt.Printf("resumed step=%d best=%.6f@%d\n", opt.StepCount, state.Best, state.BestStep)
	}
	model.train(true)
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
		ids, cont, target, f0Cont, f0Target, energyTarget := sampleBatch(training, sampler, c.Batch, c.Window)
		loss, err := update(model, opt, schedule, ids, cont, target, f0Cont, f0Target, energyTarget, c.Batch, c.Window, device, uint32(c.Seed)+uint32(step)*8, c.F0Weight, c.F0DeltaWeight, c.EnergyWeight)
		if err != nil {
			return err
		}
		if step%c.EvalEvery == 0 || step == c.Steps-1 {
			score, f0Correlation, err := evaluate(model, validation, device, vocab.silence, c.F0Weight, c.EnergyWeight)
			if err != nil {
				return err
			}
			_, planCorrelation, err := evaluate(model, planValidation, device, vocab.silence, c.F0Weight, c.EnergyWeight)
			if err != nil {
				return err
			}
			if math.IsNaN(score) || math.IsInf(score, 0) {
				return fmt.Errorf("non-finite validation score")
			}
			if score < state.Best {
				state.Best, state.BestStep = score, step
				if err := bestModel.module().LoadStateDict(model.module().StateDict()); err != nil {
					return err
				}
				if err := os.MkdirAll(filepath.Dir(c.Out), 0700); err != nil {
					return err
				}
				if err := bestModel.module().SaveSafeTensorsMetadata(c.Out, checkpointMetadata(score, step, c.TrainingCorpus, c.Notices, phones, modelIDForLanguage(c.Language), c.F0Head, c.F0Teacher != "", c.F0Dilations)); err != nil {
					return err
				}
			}
			fmt.Printf("step=%d loss=%.5f valid=%.5f f0r=%.3f f0r_plan=%.3f best=%.5f@%d elapsed=%s\n", step, loss, score, f0Correlation, planCorrelation, state.Best, state.BestStep, time.Since(started).Round(time.Second))
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
	if err := model.module().LoadStateDict(bestModel.module().StateDict()); err != nil {
		return err
	}
	if err := writeFixture(c.Fixture, model, device, len(vocab.names)); err != nil {
		return err
	}
	fmt.Printf("finished completed=%d planned=%d best_valid_l1=%.6f best_step=%d wall=%s checkpoint=%s\n", opt.StepCount, c.Steps, state.Best, state.BestStep, time.Since(started).Round(time.Millisecond), c.Checkpoint)
	return nil
}

func update(model trainerModel, opt *autograd.AdamW, schedule *autograd.OneCycle, ids []int, cv, tv, f0cv, f0tv, energyv []float32, batch, window int, device tensor.Device, dropoutSeed uint32, f0Weight, f0DeltaWeight, energyWeight float64) (float32, error) {
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
	f0Cont, err := autograd.New(f0cv, []int{batch, window, f0ContextWidth()}, device, false)
	if err != nil {
		return 0, err
	}
	defer f0Cont.Close()
	opt.ZeroGrad()
	mel, f0, energy := model.forward(ids, cont, f0Cont, dropoutSeed)
	loss := autograd.MaskedLoss(mel, target, false)
	if f0 != nil && f0Weight > 0 {
		f0Target, err := autograd.New(f0tv, []int{batch, window, 1}, device, false)
		if err != nil {
			return 0, err
		}
		defer f0Target.Close()
		absolute := autograd.MaskedLoss(f0, f0Target, false)
		loss = autograd.Add(loss, autograd.MulScalar(absolute, float32(f0Weight)))
		if f0DeltaWeight > 0 && window > 1 {
			pd := autograd.Sub(autograd.Slice(f0, 1, 1, window), autograd.Slice(f0, 1, 0, window-1))
			td := autograd.Sub(autograd.Slice(f0Target, 1, 1, window), autograd.Slice(f0Target, 1, 0, window-1))
			delta := autograd.MaskedLoss(pd, td, false)
			loss = autograd.Add(loss, autograd.MulScalar(delta, float32(f0Weight*f0DeltaWeight)))
		}
	}
	if energy != nil && energyWeight > 0 {
		energyTarget, err := autograd.New(energyv, []int{batch, window, 1}, device, false)
		if err != nil {
			return 0, err
		}
		defer energyTarget.Close()
		loss = autograd.Add(loss, autograd.MulScalar(autograd.MaskedLoss(energy, energyTarget, false), float32(energyWeight)))
	}
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

func sampleBatch(items []utterance, sampler *autograd.WindowSampler, batch, window int) ([]int, []float32, []float32, []float32, []float32, []float32) {
	ids := make([]int, batch*window*3)
	cont := make([]float32, batch*window*continuousFeatures)
	target := make([]float32, batch*window*80)
	width := f0ContextWidth()
	f0Cont := make([]float32, batch*window*width)
	f0Target := make([]float32, batch*window)
	energyTarget := make([]float32, batch*window)
	for b, sampled := range sampler.Batch() {
		item := items[sampled.Index]
		for t := 0; t < window; t++ {
			dst, src := b*window+t, sampled.Start+t
			if src >= item.Frames {
				for j := 0; j < 80; j++ {
					target[dst*80+j] = float32(math.NaN())
				}
				f0Target[dst] = float32(math.NaN())
				energyTarget[dst] = float32(math.NaN())
				continue
			}
			copy(ids[dst*3:dst*3+3], item.IDs[src*3:src*3+3])
			copy(cont[dst*continuousFeatures:(dst+1)*continuousFeatures], item.Cont[src*item.Continuous:src*item.Continuous+continuousFeatures])
			copy(target[dst*80:dst*80+80], item.Target[src*80:src*80+80])
			fillF0Context(f0Cont[dst*width:(dst+1)*width], item, src)
			f0Target[dst] = item.F0Target[src]
			energyTarget[dst] = item.EnergyTarget[src]
		}
	}
	return ids, cont, target, f0Cont, f0Target, energyTarget
}

// evaluateは検証のL1スコアと、F0の発話ごとの相関（発声フレーム、F0ヘッドがある場合）の平均を返す。
// 相関は抑揚の形がどれだけ教師に近いかの指標で、試聴前の比較に使う。
func evaluate(model trainerModel, items []utterance, device tensor.Device, silence int, f0Weight, energyWeight float64) (float64, float64, error) {
	wasTraining := model.module().Training
	model.train(false)
	defer model.train(wasTraining)
	var scores, correlations float64
	correlationCount := 0
	for _, item := range items {
		cv := make([]float32, item.Frames*continuousFeatures)
		width := f0ContextWidth()
		f0cv := make([]float32, item.Frames*width)
		for t := 0; t < item.Frames; t++ {
			copy(cv[t*continuousFeatures:(t+1)*continuousFeatures], item.Cont[t*item.Continuous:t*item.Continuous+continuousFeatures])
			fillF0Context(f0cv[t*width:(t+1)*width], item, t)
		}
		cont, err := autograd.New(cv, []int{1, item.Frames, continuousFeatures}, device, false)
		if err != nil {
			return 0, 0, err
		}
		f0Cont, err := autograd.New(f0cv, []int{1, item.Frames, width}, device, false)
		if err != nil {
			cont.Close()
			return 0, 0, err
		}
		var mel, f0, energy *autograd.Tensor
		autograd.NoGrad(func() { mel, f0, energy = model.forward(item.IDs, cont, f0Cont, 0) })
		values, err := mel.ToHost()
		mel.ReleaseGraph()
		var f0Values, energyValues []float32
		if err == nil && f0 != nil {
			f0Values, err = f0.ToHost()
			f0.ReleaseGraph()
		}
		if err == nil && energy != nil {
			energyValues, err = energy.ToHost()
			energy.ReleaseGraph()
		}
		cont.Close()
		f0Cont.Close()
		if err != nil {
			return 0, 0, err
		}
		var melTotal float64
		melCount := 0
		var f0Total float64
		f0Count := 0
		var energyTotal float64
		energyCount := 0
		var predicted, target []float64
		for t := 0; t < item.Frames; t++ {
			if item.IDs[t*3] == silence {
				continue
			}
			if f0Values != nil && !math.IsNaN(float64(item.F0Target[t])) {
				predicted = append(predicted, float64(f0Values[t]))
				target = append(target, float64(item.F0Target[t]))
			}
			for j := 0; j < 80; j++ {
				if math.IsNaN(float64(item.Target[t*80+j])) {
					continue
				}
				melTotal += math.Abs(float64(values[t*80+j] - item.Target[t*80+j]))
				melCount++
			}
			if f0Values != nil && !math.IsNaN(float64(item.F0Target[t])) {
				f0Total += math.Abs(float64(f0Values[t] - item.F0Target[t]))
				f0Count++
			}
			if energyValues != nil && !math.IsNaN(float64(item.EnergyTarget[t])) {
				energyTotal += math.Abs(float64(energyValues[t] - item.EnergyTarget[t]))
				energyCount++
			}
		}
		if melCount > 0 {
			score := melTotal / float64(melCount)
			if f0Count > 0 {
				score += f0Weight * f0Total / float64(f0Count)
			}
			if energyCount > 0 {
				score += energyWeight * energyTotal / float64(energyCount)
			}
			scores += score
		}
		if r, ok := pearson(predicted, target); ok {
			correlations += r
			correlationCount++
		}
	}
	correlation := math.NaN()
	if correlationCount > 0 {
		correlation = correlations / float64(correlationCount)
	}
	return scores / float64(len(items)), correlation, nil
}

// pearsonは2系列の相関。10点未満や分散が無い場合はfalse。
func pearson(a, b []float64) (float64, bool) {
	if len(a) < 10 || len(a) != len(b) {
		return 0, false
	}
	var meanA, meanB float64
	for i := range a {
		meanA += a[i]
		meanB += b[i]
	}
	meanA /= float64(len(a))
	meanB /= float64(len(b))
	var cov, varA, varB float64
	for i := range a {
		cov += (a[i] - meanA) * (b[i] - meanB)
		varA += (a[i] - meanA) * (a[i] - meanA)
		varB += (b[i] - meanB) * (b[i] - meanB)
	}
	if varA == 0 || varB == 0 {
		return 0, false
	}
	return cov / math.Sqrt(varA*varB), true
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
