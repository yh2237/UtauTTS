package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yh2237/gograd/autograd"
	"github.com/yh2237/gograd/cuda"
	"github.com/yh2237/gograd/tensor"
	"utautts/cmd/tools/internal/toolutil"
)

const corpus = "Tsukuyomi-chan Corpus Vol.1 (VOICEACTRESS100) + Minnade JSUT Corpus basic5000 BASIC5000_0001-0600, aligned with Montreal Forced Aligner japanese_mfa"

func main() {
	dataset := flag.String("dataset", "", "version-1 JSONL with id and audio_path")
	alignments := flag.String("alignments", "", "MFA alignment directory")
	worldEngine := flag.String("world-engine", "runtime/utautts-world-engine.dll", "WORLD engine DLL")
	cache := flag.String("cache", "out/speech-timing-target/go-features.gob", "Go feature cache")
	out := flag.String("out", filepath.Join("out", "speech-timing-target", fmt.Sprintf("go-model-%d.safetensors", time.Now().Unix())), "new checkpoint path")
	fixture := flag.String("fixture", filepath.Join("out", "speech-timing-target", fmt.Sprintf("go-parity-%d.json", time.Now().Unix())), "new parity fixture path")
	featuresOnly := flag.Bool("features-only", false, "build feature cache and exit")
	featuresJSON := flag.String("features-json", "", "write first three utterances as JSON for parity inspection")
	steps := flag.Int("steps", 6000, "training updates")
	validCount := flag.Int("valid", 30, "validation utterances")
	seed := flag.Int64("seed", 0, "model and sampler seed")
	deviceFlag := flag.String("device", "auto", "auto, cuda, or cpu")
	trainingCorpus := flag.String("training-corpus", corpus, "checkpoint corpus description")
	var notices noticeFlags
	flag.Var(&notices, "license-notice", "license notice path (repeatable)")
	flag.Parse()
	if *dataset == "" || *alignments == "" {
		fatal("--dataset and --alignments are required")
	}
	if *steps < 2 {
		fatal("steps must be >= 2")
	}
	if _, e := os.Stat(*out); e == nil {
		fatal("refusing to overwrite %s", *out)
	}
	if _, e := os.Stat(*fixture); e == nil {
		fatal("refusing to overwrite %s", *fixture)
	}
	items, e := loadOrBuildFeatures(*cache, *dataset, *alignments, *worldEngine)
	if e != nil {
		fatal("read features: %v", e)
	}
	if *featuresJSON != "" {
		if _, e := os.Stat(*featuresJSON); e == nil {
			fatal("refusing to overwrite %s", *featuresJSON)
		}
		n := min(3, len(items))
		b, e := json.Marshal(items[:n])
		if e != nil {
			fatal("marshal features: %v", e)
		}
		if e = os.MkdirAll(filepath.Dir(*featuresJSON), 0755); e != nil {
			fatal("feature export directory: %v", e)
		}
		file, e := toolutil.CreateExclusive(*featuresJSON)
		if e != nil {
			fatal("feature export: %v", e)
		}
		_, e = file.Write(b)
		closeErr := file.Close()
		if e != nil {
			fatal("feature export: %v", e)
		}
		if closeErr != nil {
			fatal("feature export: %v", closeErr)
		}
	}
	if *featuresOnly {
		fmt.Printf("utterances=%d frames=%d cache=%s\n", len(items), totalFrames(items), *cache)
		return
	}
	if *validCount < 1 || *validCount >= len(items) {
		fatal("invalid validation count")
	}
	continuous := 4
	for _, item := range items {
		if item.Continuous < continuous {
			fatal("cache %s has %d continuous features, need %d", item.ID, item.Continuous, continuous)
		}
	}
	device, e := toolutil.ResolveDevice(*deviceFlag)
	if e != nil {
		fatal("%v", e)
	}
	if device == tensor.CUDA {
		ctx, e := autograd.NewCUDAContext()
		if e != nil {
			fatal("CUDA: %v", e)
		}
		defer ctx.Close()
	}
	rng := rand.New(rand.NewSource(*seed))
	order := rng.Perm(len(items))
	fmt.Println("split=Go RNG")
	validation := make([]utterance, *validCount)
	training := make([]utterance, len(items)-*validCount)
	for i, index := range order {
		if i < *validCount {
			validation[i] = items[index]
		} else {
			training[i-*validCount] = items[index]
		}
	}
	model, e := autograd.NewSpeechTiming(continuous, device, *seed)
	if e != nil {
		fatal("model: %v", e)
	}
	params := model.Parameters()
	opt := autograd.NewAdamW(params, .002, .0001)
	schedule := autograd.NewOneCycle(.002, *steps, .1)
	opt.LR = float32(schedule.LR())
	startTime := time.Now()
	best := math.Inf(1)
	bestStep := -1
	fmt.Printf("device=%s utterances=%d train=%d valid=%d frames=%d steps=%d output=%s\n", device, len(items), len(training), len(validation), totalFrames(items), *steps, *out)
	for step := 0; step < *steps; step++ {
		ids, cv, tv := sampleBatch(training, rng, continuous)
		cont, e := autograd.New(cv, []int{16, 400, continuous}, device, false)
		if e != nil {
			fatal("input: %v", e)
		}
		target, e := autograd.New(tv, []int{16, 400, 80}, device, false)
		if e != nil {
			fatal("target: %v", e)
		}
		opt.ZeroGrad()
		pred := model.Forward(ids, cont, uint32(*seed)+uint32(step*8))
		loss := autograd.MaskedLoss(pred, target, false)
		lossValue, e := loss.ToHost()
		if e != nil {
			fatal("loss: %v", e)
		}
		if e = loss.Backward(); e != nil {
			fatal("backward: %v", e)
		}
		autograd.ClipGradNorm(params, 1)
		opt.Step()
		schedule.Step(opt)
		loss.ReleaseGraph()
		cont.Close()
		target.Close()
		if step%250 == 0 || step == *steps-1 {
			score := evaluate(model, validation, continuous, device)
			if score < best {
				best, bestStep = score, step
				meta := checkpointMetadata(best, bestStep, *trainingCorpus, notices)
				if e = os.MkdirAll(filepath.Dir(*out), 0700); e != nil {
					fatal("output directory: %v", e)
				}
				if e = model.Module.SaveSafeTensorsMetadata(*out, meta); e != nil {
					fatal("checkpoint: %v", e)
				}
			}
			fmt.Printf("step=%d loss=%.5f valid=%.5f best=%.5f@%d elapsed=%s\n", step, lossValue[0], score, best, bestStep, time.Since(startTime).Round(time.Second))
		}
	}
	if e = model.Module.LoadSafeTensors(*out); e != nil {
		fatal("reload best checkpoint: %v", e)
	}
	if e = writeFixture(*fixture, model, device); e != nil {
		fatal("fixture: %v", e)
	}
	if device == tensor.CUDA {
		if e := cuda.Synchronize(); e != nil {
			fatal("CUDA sync: %v", e)
		}
	}
	fmt.Printf("finished steps=%d best_valid_l1=%.6f best_step=%d wall=%s checkpoint=%s\n", *steps, best, bestStep, time.Since(startTime).Round(time.Millisecond), *out)
}
func fatal(format string, args ...any) { fmt.Fprintf(os.Stderr, format+"\n", args...); os.Exit(1) }
func totalFrames(items []utterance) int {
	n := 0
	for _, x := range items {
		n += x.Frames
	}
	return n
}

func sampleBatch(items []utterance, rng *rand.Rand, c int) ([]int, []float32, []float32) {
	ids := make([]int, 16*400*3)
	cont := make([]float32, 16*400*c)
	target := make([]float32, 16*400*80)
	for b := 0; b < 16; b++ {
		item := items[rng.Intn(len(items))]
		choices := item.Frames - 400
		if choices < 1 {
			choices = 1
		}
		start := rng.Intn(choices)
		for t := 0; t < 400; t++ {
			dst := (b*400 + t)
			src := start + t
			if src >= item.Frames {
				for j := 0; j < 80; j++ {
					target[dst*80+j] = float32(math.NaN())
				}
				continue
			}
			copy(ids[dst*3:dst*3+3], item.IDs[src*3:src*3+3])
			copy(cont[dst*c:dst*c+c], item.Cont[src*item.Continuous:src*item.Continuous+c])
			copy(target[dst*80:dst*80+80], item.Target[src*80:src*80+80])
		}
	}
	return ids, cont, target
}
func evaluate(model *autograd.SpeechTiming, items []utterance, c int, device tensor.Device) float64 {
	model.Train(false)
	defer model.Train(true)
	var scores float64
	for _, item := range items {
		cv := make([]float32, item.Frames*c)
		for t := 0; t < item.Frames; t++ {
			copy(cv[t*c:(t+1)*c], item.Cont[t*item.Continuous:t*item.Continuous+c])
		}
		cont, e := autograd.New(cv, []int{1, item.Frames, c}, device, false)
		if e != nil {
			fatal("validation input: %v", e)
		}
		var pred *autograd.Tensor
		autograd.NoGrad(func() { pred = model.Forward(item.IDs, cont, 0) })
		values, e := pred.ToHost()
		if e != nil {
			fatal("validation output: %v", e)
		}
		var total float64
		count := 0
		for t := 0; t < item.Frames; t++ {
			if item.IDs[t*3] == 2 {
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
		pred.ReleaseGraph()
		cont.Close()
	}
	return scores / float64(len(items))
}
func checkpointMetadata(score float64, step int, trainingCorpus string, notices []string) map[string]string {
	if len(notices) == 0 {
		notices = []string{"licenses/TSUKUYOMI-CORPUS.txt", "licenses/MINNADE-JSUT-CORPUS.txt", "licenses/MFA-Japanese-NOTICE.txt"}
	}
	return map[string]string{"id": "speech-timing-target-v1", "format": "utautts-speech-timing-tcn-1", "phones": phoneNames,
		"kernel": "5", "dilations": "1 2 4 8 1 2 4 8", "frame_ms": "10.0", "mels": "80",
		"license": "MIT License", "training_corpus": trainingCorpus, "license_notices": strings.Join(notices, " "),
		"valid_l1": fmt.Sprintf("%.4f", score), "steps": fmt.Sprint(step)}
}

type noticeFlags []string

func (n *noticeFlags) String() string     { return strings.Join(*n, ",") }
func (n *noticeFlags) Set(s string) error { *n = append(*n, s); return nil }
