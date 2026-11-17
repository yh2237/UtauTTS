package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/yh2237/UtauTTS/cmd/tools/internal/f0cache"
	"github.com/yh2237/UtauTTS/cmd/tools/internal/toolutil"
)

type token struct {
	Start float64 `json:"start_ms"`
	End   float64 `json:"end_ms"`
	Pause bool    `json:"pause"`
}
type record struct {
	ID        string  `json:"id"`
	RecordID  string  `json:"record_id"`
	AudioPath string  `json:"audio_path"`
	Start     float64 `json:"start_ms"`
	End       float64 `json:"end_ms"`
	Tokens    []token `json:"tokens"`
}

func records(path string) ([]record, error) {
	var rows []record
	err := toolutil.ScanJSONL(path, func(line []byte) error {
		var r record
		if e := json.Unmarshal(line, &r); e != nil {
			return e
		}
		if r.ID == "" || r.AudioPath == "" || len(r.Tokens) == 0 {
			return fmt.Errorf("invalid record %q", r.ID)
		}
		rows = append(rows, r)
		return nil
	})
	return rows, err
}

func (t token) F0Timing() (float64, float64, bool) {
	return t.Start, t.End, t.Pause
}
func cachePath(dir string, r record) string {
	return f0cache.Path(dir, r.ID, r.RecordID, r.Tokens, 10, f0cache.WorldHarvest)
}
func times(r record) []float64 {
	return f0cache.TimeGrid(r.Start, r.End, r.Tokens, 10)
}
func tokenAt(ts []token, t float64) int {
	return f0cache.TokenAt(ts, t)
}
func interpolate(track []float64, ts []float64) []float64 {
	return f0cache.Interpolate(track, ts, 10)
}
func resolveAudio(path, dataset string) string {
	norm := strings.ReplaceAll(path, "\\", string(filepath.Separator))
	if filepath.IsAbs(norm) {
		return norm
	}
	for _, candidate := range []string{path, norm, filepath.Join(filepath.Dir(dataset), norm), filepath.Join(filepath.Dir(filepath.Dir(dataset)), norm)} {
		if info, e := os.Stat(candidate); e == nil && !info.IsDir() {
			return candidate
		}
	}
	base, _ := filepath.Abs(filepath.Dir(dataset))
	for {
		candidate := filepath.Join(base, norm)
		if info, e := os.Stat(candidate); e == nil && !info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(base)
		if parent == base {
			break
		}
		base = parent
	}
	return path
}
func writeNPY(w io.Writer, x []float64) error {
	return f0cache.WriteNPY(w, x, 64)
}
func cacheOne(r record, dataset, dir, engine string) (bool, error) {
	path := cachePath(dir, r)
	if _, e := os.Stat(path); e == nil {
		return false, nil
	}
	r.AudioPath = resolveAudio(r.AudioPath, dataset)
	track, e := f0cache.WorldF0(r.AudioPath, 10, engine)
	if e != nil {
		return false, e
	}
	ts := times(r)
	f0 := interpolate(track, ts)
	for i, t := range ts {
		if r.Tokens[tokenAt(r.Tokens, t)].Pause {
			f0[i] = 0
		}
		if f0[i] < 0 {
			f0[i] = 0
		}
	}
	tmp, e := os.CreateTemp(dir, ".f0-*.npy")
	if e != nil {
		return false, e
	}
	defer os.Remove(tmp.Name())
	if e = writeNPY(tmp, f0); e != nil {
		tmp.Close()
		return false, e
	}
	if e = tmp.Close(); e != nil {
		return false, e
	}
	if _, e = os.Stat(path); e == nil {
		return false, nil
	}
	if e = os.Rename(tmp.Name(), path); e != nil {
		return false, e
	}
	return true, nil
}
func run(dataset, dir, engine string, workers int) (int, error) {
	if workers < 1 {
		return 0, errors.New("workers must be positive")
	}
	if !toolutil.UnderOut(dir) {
		return 0, errors.New("cache must be under out/")
	}
	rows, e := records(dataset)
	if e != nil {
		return 0, e
	}
	if e = os.MkdirAll(dir, 0755); e != nil {
		return 0, e
	}
	jobs := make(chan record)
	errs := make(chan error, len(rows))
	var wg sync.WaitGroup
	var mu sync.Mutex
	done, written := 0, 0
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := range jobs {
				made, err := cacheOne(r, dataset, dir, engine)
				mu.Lock()
				done++
				if made {
					written++
				}
				if done%25 == 0 || done == len(rows) {
					fmt.Printf("F0 cache %d/%d\n", done, len(rows))
				}
				mu.Unlock()
				if err != nil {
					errs <- fmt.Errorf("%s: %w", r.ID, err)
				}
			}
		}()
	}
	for _, r := range rows {
		jobs <- r
	}
	close(jobs)
	wg.Wait()
	close(errs)
	for err := range errs {
		return written, err
	}
	return written, nil
}
func main() {
	dataset := flag.String("dataset", "", "version-1 JSONL")
	dir := flag.String("cache", "", "F0 cache under out/")
	engine := flag.String("world-engine", "runtime/utautts-world-engine.dll", "WORLD engine DLL")
	workers := flag.Int("workers", 4, "parallel workers")
	flag.Parse()
	if *dataset == "" || *dir == "" {
		fmt.Fprintln(os.Stderr, "--dataset and --cache are required")
		os.Exit(2)
	}
	written, e := run(*dataset, *dir, *engine, *workers)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	fmt.Printf("cached %d new records\n", written)
}
