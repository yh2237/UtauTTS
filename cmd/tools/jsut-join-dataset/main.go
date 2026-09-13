// jsut-join-datasetはjsut-labelを音素単位の学習データへ変換する
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"utautts/internal/audio"
	"utautts/internal/jsut"
)

func main() {
	var (
		labelsDir string
		corpusDir string
		outPath   string
		limit     int
		frameMS   float64
		skipAudio bool
		force     bool
	)
	flag.StringVar(&labelsDir, "labels", "", "jsut-label directory containing .lab files")
	flag.StringVar(&corpusDir, "corpus", "", "BASIC5000 directory containing wav/ and transcript_utf8.txt")
	flag.StringVar(&outPath, "out", "", "output JSONL path, or - for stdout")
	flag.IntVar(&limit, "limit", 0, "maximum number of label files (0 means all)")
	flag.Float64Var(&frameMS, "frame-ms", 30, "analysis window length in milliseconds")
	flag.BoolVar(&skipAudio, "skip-audio-features", false, "write timings without reading WAV files")
	flag.BoolVar(&force, "force", false, "overwrite an existing output file")
	flag.Parse()
	if labelsDir == "" || corpusDir == "" || outPath == "" {
		flag.Usage()
		os.Exit(2)
	}
	if limit < 0 {
		fail(fmt.Errorf("limit must be non-negative"))
	}
	if frameMS <= 0 {
		fail(fmt.Errorf("frame-ms must be positive"))
	}
	transcripts, err := readTranscripts(filepath.Join(corpusDir, "transcript_utf8.txt"))
	if err != nil {
		fail(err)
	}
	labelPaths, err := findLabels(labelsDir)
	if err != nil {
		fail(err)
	}
	if limit > 0 && len(labelPaths) > limit {
		labelPaths = labelPaths[:limit]
	}
	writer, closeWriter, err := openOutput(outPath, force)
	if err != nil {
		fail(err)
	}
	defer func() {
		if closeErr := closeWriter(); closeErr != nil {
			fail(closeErr)
		}
	}()
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	var recordCount, phoneCount, boundaryCount, featureCount int
	for _, labelPath := range labelPaths {
		id := strings.TrimSuffix(filepath.Base(labelPath), filepath.Ext(labelPath))
		text, ok := transcripts[id]
		if !ok {
			fail(fmt.Errorf("%s: transcript is missing", id))
		}
		labelData, err := os.ReadFile(labelPath)
		if err != nil {
			fail(fmt.Errorf("read labels %s: %w", labelPath, err))
		}
		phones, boundaries, err := jsut.ParseHTSLabels(string(labelData))
		if err != nil {
			fail(fmt.Errorf("%s: %w", id, err))
		}
		wavPath := filepath.Join(corpusDir, "wav", id+".wav")
		if _, err := os.Stat(wavPath); err != nil {
			fail(fmt.Errorf("%s: missing audio %s: %w", id, wavPath, err))
		}
		audioPath, err := filepath.Abs(wavPath)
		if err != nil {
			fail(fmt.Errorf("%s: resolve audio path: %w", id, err))
		}
		record := jsut.NewAlignment(id, text, audioPath, phones, boundaries)
		if !skipAudio {
			pcm, err := audio.ReadWav(wavPath)
			if err != nil {
				fail(fmt.Errorf("%s: read audio: %w", id, err))
			}
			if err := jsut.AttachAudioFeatures(&record, pcm, frameMS); err != nil {
				fail(fmt.Errorf("%s: analyze audio: %w", id, err))
			}
			for _, phone := range record.Phones {
				if phone.Frame != nil && phone.Frame.Valid {
					featureCount++
				}
			}
			for _, boundary := range record.Boundaries {
				if boundary.Features != nil {
					featureCount++
				}
			}
		}
		if err := record.Validate(); err != nil {
			fail(fmt.Errorf("%s: %w", id, err))
		}
		if err := encoder.Encode(record); err != nil {
			fail(fmt.Errorf("write %s: %w", outPath, err))
		}
		recordCount++
		phoneCount += len(record.Phones)
		for _, boundary := range record.Boundaries {
			if boundary.Trainable {
				boundaryCount++
			}
		}
	}
	fmt.Fprintf(os.Stderr, "jsut join dataset: %d utterances, %d phones, %d trainable boundaries, %d acoustic observations\n", recordCount, phoneCount, boundaryCount, featureCount)
}

func findLabels(root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("labels: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("labels is not a directory: %s", root)
	}
	paths := make([]string, 0, 5000)
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".lab") {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("find labels: %w", err)
	}
	sort.Slice(paths, func(i, j int) bool {
		return naturalName(filepath.Base(paths[i])) < naturalName(filepath.Base(paths[j]))
	})
	if len(paths) == 0 {
		return nil, fmt.Errorf("no .lab files found in %s", root)
	}
	return paths, nil
}

func naturalName(name string) string {
	parts := strings.Split(name, "_")
	if len(parts) == 2 {
		if number, err := strconv.Atoi(strings.TrimSuffix(parts[1], filepath.Ext(parts[1]))); err == nil {
			return fmt.Sprintf("%s_%08d", parts[0], number)
		}
	}
	return name
}

func readTranscripts(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read transcript %s: %w", path, err)
	}
	defer file.Close()
	result := make(map[string]string)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 2*1024*1024)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimPrefix(strings.TrimSpace(scanner.Text()), "\ufeff")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		id, text, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(id) == "" {
			return nil, fmt.Errorf("transcript line %d: expected id:text", lineNumber)
		}
		id = strings.TrimSpace(id)
		if _, exists := result[id]; exists {
			return nil, fmt.Errorf("transcript line %d: duplicate id %q", lineNumber, id)
		}
		result[id] = strings.TrimSpace(text)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read transcript %s: %w", path, err)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("transcript %s is empty", path)
	}
	return result, nil
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
