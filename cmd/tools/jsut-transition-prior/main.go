// jsut-transition-priorは遷移差分を音素対ごとに集計して評価する。
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"utautts/internal/jsut"
)

func main() {
	input := flag.String("input", "", "transition dataset JSONL path")
	out := flag.String("out", "", "output prior JSON path")
	bins := flag.Int("bins", 15, "normalized position bins")
	validationPercent := flag.Int("validation-percent", 20, "utterance validation percentage")
	force := flag.Bool("force", false, "overwrite output")
	flag.Parse()
	if *input == "" || *out == "" || *validationPercent < 1 || *validationPercent > 50 {
		flag.Usage()
		os.Exit(2)
	}
	examples, err := jsut.ReadTransitionExamples(*input)
	if err != nil {
		fail(err)
	}
	var train, validation []jsut.TransitionExample
	for _, example := range examples {
		hash := sha256.Sum256([]byte(example.UtteranceID))
		if int(hash[0])%100 < *validationPercent {
			validation = append(validation, example)
		} else {
			train = append(train, example)
		}
	}
	if len(train) == 0 || len(validation) == 0 {
		fail(fmt.Errorf("training or validation split is empty"))
	}
	prior, err := jsut.BuildTransitionPrior(train, *bins, "")
	if err != nil {
		fail(err)
	}
	metrics := jsut.EvaluateTransitionPrior(prior, validation)
	payload := struct {
		Model      *jsut.TransitionPrior  `json:"model"`
		Validation jsut.TransitionMetrics `json:"validation"`
	}{prior, metrics}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		fail(err)
	}
	if directory := filepath.Dir(*out); directory != "." {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			fail(err)
		}
	}
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if *force {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	file, err := os.OpenFile(*out, flags, 0o644)
	if err != nil {
		fail(fmt.Errorf("open %s: %w", *out, err))
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		_ = file.Close()
		fail(err)
	}
	if err := file.Close(); err != nil {
		fail(err)
	}
	fmt.Printf("train=%d validation=%d spectrum %.4f -> %.4f dB rms %.4f -> %.4f dB\n", len(train), len(validation), metrics.BaselineSpectrumMAE, metrics.PriorSpectrumMAE, metrics.BaselineRMSMAE, metrics.PriorRMSMAE)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
