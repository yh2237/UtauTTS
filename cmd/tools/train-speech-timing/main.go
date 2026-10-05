package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"
)

const corpus = "Tsukuyomi-chan Corpus Vol.1 (VOICEACTRESS100) + Minnade JSUT Corpus basic5000 BASIC5000_0001-0600, aligned with Montreal Forced Aligner japanese_mfa"

type trainingConfig struct {
	Dataset, Alignments, WorldEngine, Cache, Out, Fixture, FeaturesJSON string
	Checkpoint, Resume, Device, TrainingCorpus                          string
	Steps, StopAfter, Valid, Batch, Window, EvalEvery, CheckpointEvery  int
	Seed                                                                int64
	FeaturesOnly                                                        bool
	Notices                                                             noticeFlags
}

func main() {
	var c trainingConfig
	stamp := time.Now().Unix()
	flag.StringVar(&c.Dataset, "dataset", "", "version-1 JSONL with id and audio_path")
	flag.StringVar(&c.Alignments, "alignments", "", "MFA alignment directory")
	flag.StringVar(&c.WorldEngine, "world-engine", "runtime/utautts-world-engine.dll", "WORLD engine DLL")
	flag.StringVar(&c.Cache, "cache", "out/speech-timing-target/go-features.gob", "Go feature cache")
	flag.StringVar(&c.Out, "out", filepath.Join("out", "speech-timing-target", fmt.Sprintf("go-model-%d.safetensors", stamp)), "new best inference weights path")
	flag.StringVar(&c.Fixture, "fixture", filepath.Join("out", "speech-timing-target", fmt.Sprintf("go-parity-%d.json", stamp)), "new parity fixture path for best weights")
	flag.BoolVar(&c.FeaturesOnly, "features-only", false, "build feature cache and exit")
	flag.StringVar(&c.FeaturesJSON, "features-json", "", "write first three utterances as JSON for parity inspection")
	flag.IntVar(&c.Steps, "steps", 6000, "total planned updates; keep unchanged when resuming")
	flag.IntVar(&c.Valid, "valid", 30, "validation utterances")
	flag.Int64Var(&c.Seed, "seed", 0, "model, split and sampler seed")
	flag.StringVar(&c.Device, "device", "auto", "auto, cuda, or cpu")
	flag.StringVar(&c.TrainingCorpus, "training-corpus", corpus, "checkpoint corpus description")
	flag.Var(&c.Notices, "license-notice", "license notice path (repeatable)")
	flag.StringVar(&c.Checkpoint, "checkpoint", "", "resumable training state (default: out + .training.safetensors)")
	flag.StringVar(&c.Resume, "resume", "", "resume from a training checkpoint, not inference weights")
	flag.IntVar(&c.StopAfter, "stop-after", 0, "stop at this completed update and save training state (0: all planned updates)")
	flag.IntVar(&c.CheckpointEvery, "checkpoint-every", 250, "training checkpoint interval in completed updates")
	flag.IntVar(&c.EvalEvery, "eval-every", 250, "validation interval (also first/final update)")
	flag.IntVar(&c.Batch, "batch-size", 16, "windows per training batch")
	flag.IntVar(&c.Window, "window", 400, "frames per sampled window")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := train(ctx, c); err != nil {
		fmt.Fprintln(os.Stderr, "train-speech-timing:", err)
		os.Exit(1)
	}
}

func totalFrames(items []utterance) int {
	n := 0
	for _, x := range items {
		n += x.Frames
	}
	return n
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
