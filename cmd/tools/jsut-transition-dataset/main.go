// jsut-transition-datasetは自然境界から単独音の遷移補完データを作る。
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"utautts/internal/audio"
	"utautts/internal/jsut"
)

func main() {
	input := flag.String("input", "", "jsut-join-dataset JSONL path")
	out := flag.String("out", "", "output JSONL path")
	windowMS := flag.Float64("window-ms", 40, "maximum milliseconds on each side of a boundary")
	stepMS := flag.Float64("step-ms", 5, "feature step in milliseconds")
	limit := flag.Int("limit", 0, "maximum utterances (0 means all)")
	force := flag.Bool("force", false, "overwrite output")
	flag.Parse()
	if *input == "" || *out == "" || *limit < 0 {
		flag.Usage()
		os.Exit(2)
	}
	records, err := jsut.ReadAlignments(*input)
	if err != nil {
		fail(err)
	}
	if *limit > 0 && len(records) > *limit {
		records = records[:*limit]
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
		fail(err)
	}
	writer := bufio.NewWriter(file)
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	count := 0
	for _, record := range records {
		pcm, err := audio.ReadWav(record.AudioPath)
		if err != nil {
			fail(fmt.Errorf("%s: %w", record.ID, err))
		}
		examples, err := jsut.BuildTransitionExamples(record, pcm, *windowMS, *stepMS)
		if err != nil {
			fail(fmt.Errorf("%s: %w", record.ID, err))
		}
		for _, example := range examples {
			if err := encoder.Encode(example); err != nil {
				fail(err)
			}
			count++
		}
	}
	if err := writer.Flush(); err != nil {
		fail(err)
	}
	if err := file.Close(); err != nil {
		fail(err)
	}
	fmt.Fprintf(os.Stderr, "jsut transition dataset: %d utterances, %d boundaries -> %s\n", len(records), count, *out)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
