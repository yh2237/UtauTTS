// cache-frame-f0 precomputes the Python trainer's WORLD Harvest F0 cache.
package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 4096), 16<<20)
	var rows []record
	for s.Scan() {
		if strings.TrimSpace(s.Text()) == "" {
			continue
		}
		var r record
		if e = json.Unmarshal(s.Bytes(), &r); e != nil {
			return nil, e
		}
		if r.ID == "" || r.AudioPath == "" || len(r.Tokens) == 0 {
			return nil, fmt.Errorf("invalid record %q", r.ID)
		}
		rows = append(rows, r)
	}
	return rows, s.Err()
}

func pyFloat(v float64) string {
	s := strconv.FormatFloat(math.Round(v*1000)/1000, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}
func cachePath(dir string, r record) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, t := range r.Tokens {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("[" + pyFloat(t.Start) + ", " + pyFloat(t.End) + ", " + map[bool]string{true: "true", false: "false"}[t.Pause] + "]")
	}
	b.WriteByte(']')
	id := r.RecordID
	if id == "" {
		id = r.ID
	}
	key := fmt.Sprintf("%s|10|utautts_world_harvest|%s", id, b.String())
	h := sha1.Sum([]byte(key))
	return filepath.Join(dir, fmt.Sprintf("%x.npy", h))
}
func times(r record) []float64 {
	start, end := r.Start, r.End
	if end <= start {
		start = r.Tokens[0].Start
		end = r.Tokens[len(r.Tokens)-1].End
	}
	n := max(1, int(math.Ceil((end-start)/10)))
	out := make([]float64, n)
	for i := range out {
		out[i] = start + (float64(i)+.5)*10
	}
	return out
}
func tokenAt(ts []token, t float64) int {
	for i, x := range ts {
		if x.Start <= t && t < math.Max(x.Start+1e-6, x.End) {
			return i
		}
	}
	if t < ts[0].Start {
		return 0
	}
	return len(ts) - 1
}
func interpolate(track []float64, ts []float64) []float64 {
	out := make([]float64, len(ts))
	for i, t := range ts {
		p := t / 10
		l := int(math.Floor(p))
		if l < 0 || l >= len(track) {
			continue
		}
		if math.Abs(p-float64(l)) < 1e-9 {
			out[i] = track[l]
			continue
		}
		h := l + 1
		if h >= len(track) || track[l] <= 0 || track[h] <= 0 {
			continue
		}
		w := p - float64(l)
		out[i] = math.Exp(math.Log(track[l])*(1-w) + math.Log(track[h])*w)
	}
	return out
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
	header := "{'descr': '<f8', 'fortran_order': False, 'shape': (" + fmt.Sprint(len(x)) + ",), }"
	pad := 64 - (10+len(header)+1)%64
	header += strings.Repeat(" ", pad) + "\n"
	pre := append([]byte("\x93NUMPY\x01\x00"), byte(len(header)), byte(len(header)>>8))
	pre = append(pre, header...)
	if _, e := w.Write(pre); e != nil {
		return e
	}
	return binary.Write(w, binary.LittleEndian, x)
}
func cacheOne(r record, dataset, dir, engine string) (bool, error) {
	path := cachePath(dir, r)
	if _, e := os.Stat(path); e == nil {
		return false, nil
	}
	r.AudioPath = resolveAudio(r.AudioPath, dataset)
	track, e := worldF0(r, 10, engine)
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
	clean := filepath.Clean(dir)
	if clean != "out" && !strings.HasPrefix(clean, "out"+string(filepath.Separator)) {
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
