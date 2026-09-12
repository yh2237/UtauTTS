// join-ranker trains the optional dependency-free join-quality model.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"utautts/internal/connection"
)

type stringList []string

func (values *stringList) String() string {
	return strings.Join(*values, ", ")
}

func (values *stringList) Set(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("input path must not be empty")
	}
	*values = append(*values, value)
	return nil
}

func main() {
	var inputs stringList
	var (
		outPath       string
		modelID       string
		description   string
		epochs        int
		learningRate  float64
		l2            float64
		blend         float64
		scoreScale    float64
		minConfidence float64
	)
	flag.Var(&inputs, "input", "join-audit JSON or JSONL path (repeatable)")
	flag.StringVar(&outPath, "out", "", "output model JSON path")
	flag.StringVar(&modelID, "id", "join-ranker-v1", "model ID")
	flag.StringVar(&description, "description", "", "model description")
	flag.IntVar(&epochs, "epochs", 1200, "training epochs")
	flag.Float64Var(&learningRate, "learning-rate", 0.08, "logistic gradient descent learning rate")
	flag.Float64Var(&l2, "l2", 0.001, "L2 regularization")
	flag.Float64Var(&blend, "blend", 0.35, "runtime correction blend (0..1)")
	flag.Float64Var(&scoreScale, "score-scale", 8, "maximum learned join correction")
	flag.Float64Var(&minConfidence, "min-confidence", 0.08, "minimum probability confidence for runtime correction")
	flag.Parse()
	if len(inputs) == 0 || outPath == "" {
		flag.Usage()
		os.Exit(2)
	}

	rows := make([]connection.JoinAuditRow, 0)
	for _, path := range inputs {
		loaded, err := connection.ReadJoinAuditRows(path)
		if err != nil {
			fail(err)
		}
		rows = append(rows, loaded...)
	}
	model, report, err := connection.TrainJoinModel(rows, connection.JoinTrainingOptions{
		ID: modelID, Description: description, Epochs: epochs, LearningRate: learningRate, L2: l2,
		Blend: blend, ScoreScale: scoreScale, MinConfidence: minConfidence,
	})
	if err != nil {
		fail(err)
	}
	data, err := json.MarshalIndent(model, "", "  ")
	if err != nil {
		fail(err)
	}
	data = append(data, '\n')
	if directory := filepath.Dir(outPath); directory != "." {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			fail(err)
		}
	}
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		fail(err)
	}
	fmt.Fprintf(os.Stderr, "join model: %d examples positive=%d negative=%d accuracy=%.3f logloss=%.4f -> %s\n",
		report.Examples, report.Positive, report.Negative, report.Accuracy, report.LogLoss, outPath)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
