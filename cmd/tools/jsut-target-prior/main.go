// jsut-target-priorはJSUTの音素目標と自然境界を集計する
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"utautts/internal/jsut"
)

type inputPaths []string

func (paths *inputPaths) String() string { return strings.Join(*paths, ", ") }

func (paths *inputPaths) Set(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("input path must not be empty")
	}
	*paths = append(*paths, value)
	return nil
}

func main() {
	var inputs inputPaths
	var (
		outPath       string
		modelID       string
		description   string
		includePauses bool
		force         bool
	)
	flag.Var(&inputs, "input", "alignment JSONL path (repeatable)")
	flag.StringVar(&outPath, "out", "", "output prior JSON path, or - for stdout")
	flag.StringVar(&modelID, "id", "jsut-target-prior-v1", "prior model ID")
	flag.StringVar(&description, "description", "", "prior description")
	flag.BoolVar(&includePauses, "include-pauses", false, "include sil and pau in phone statistics")
	flag.BoolVar(&force, "force", false, "overwrite an existing output file")
	flag.Parse()
	if len(inputs) == 0 || outPath == "" {
		flag.Usage()
		os.Exit(2)
	}
	records := make([]jsut.Alignment, 0)
	for _, path := range inputs {
		loaded, err := jsut.ReadAlignments(path)
		if err != nil {
			fail(err)
		}
		records = append(records, loaded...)
	}
	prior, err := jsut.BuildPrior(records, includePauses, modelID, description)
	if err != nil {
		fail(err)
	}
	data, err := json.MarshalIndent(prior, "", "  ")
	if err != nil {
		fail(err)
	}
	data = append(data, '\n')
	writer, closeWriter, err := openOutput(outPath, force)
	if err != nil {
		fail(err)
	}
	if _, err := writer.Write(data); err != nil {
		_ = closeWriter()
		fail(fmt.Errorf("write %s: %w", outPath, err))
	}
	if err := closeWriter(); err != nil {
		fail(err)
	}
	fmt.Fprintf(os.Stderr, "jsut target prior: %d utterances, %d phones, %d natural boundaries -> %s\n", prior.Utterances, prior.PhoneCount, prior.BoundaryCount, outPath)
}

func openOutput(path string, force bool) (io.Writer, func() error, error) {
	if path == "-" {
		return os.Stdout, func() error { return nil }, nil
	}
	if directory := filepath.Dir(path); directory != "." {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return nil, nil, err
		}
	}
	flags := os.O_WRONLY | os.O_CREATE
	if force {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}
	file, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("open output %s: %w (use --force to overwrite)", path, err)
	}
	return file, file.Close, nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
