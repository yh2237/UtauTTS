package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"utautts/cmd/tools/internal/toolutil"
)

func findDir(root, part, suffix string) (string, error) {
	var found string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if found != "" {
			return fs.SkipAll
		}
		if !d.IsDir() || !strings.Contains(d.Name(), part) {
			return nil
		}
		if suffix == "" {
			found = path
			return fs.SkipAll
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), suffix) {
				found = path
				return fs.SkipAll
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("directory containing %q not found below %s", part, root)
	}
	return found, nil
}
func textFile(root, part string) (string, error) {
	var found string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() && strings.Contains(d.Name(), part) && strings.HasSuffix(d.Name(), ".txt") {
			found = path
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("script %q not found", part)
	}
	return found, nil
}
func original(path string) (map[string]string, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	out := map[string]string{}
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimRight(s.Text(), "\r\n ")
		if strings.HasPrefix(line, "BASIC5000_") {
			key, value, ok := strings.Cut(line, ":")
			if ok {
				out[key] = strings.TrimSpace(value)
			}
		}
	}
	return out, s.Err()
}
func readings(path string) (map[string]string, map[string]string, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, nil, e
	}
	defer f.Close()
	texts, kanas := map[string]string{}, map[string]string{}
	current := ""
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimRight(s.Text(), "\r\n ")
		if strings.HasPrefix(line, "BASIC5000_") && strings.HasSuffix(line, ":") {
			current = strings.TrimSuffix(line, ":")
		} else if current != "" {
			if _, value, ok := strings.Cut(line, "text_level0:"); ok {
				texts[current] = strings.TrimSpace(value)
			}
			if _, value, ok := strings.Cut(line, "kana_level0:"); ok {
				kanas[current] = strings.TrimSpace(value)
			}
		}
	}
	return texts, kanas, s.Err()
}
func run(root, out, audioDir string, start, end int) (int, error) {
	if start < 1 || end < start {
		return 0, fmt.Errorf("invalid range")
	}
	if !toolutil.UnderOutChild(out) {
		return 0, fmt.Errorf("output must be under out/")
	}
	if _, e := os.Stat(out); e == nil {
		return 0, fmt.Errorf("refusing to overwrite %s", out)
	}
	var e error
	if audioDir == "" {
		audioDir, e = findDir(root, "01 ", ".wav")
		if e != nil {
			return 0, e
		}
	}
	textDir, e := findDir(root, "02 ", "")
	if e != nil {
		return 0, e
	}
	originalPath, e := textFile(textDir, "オリジナル")
	if e != nil {
		return 0, e
	}
	readingPath, e := textFile(textDir, "読み仮名")
	if e != nil {
		return 0, e
	}
	originalTexts, e := original(originalPath)
	if e != nil {
		return 0, e
	}
	readingTexts, readingKanas, e := readings(readingPath)
	if e != nil {
		return 0, e
	}
	if e = os.MkdirAll(filepath.Join(out, "wavs"), 0755); e != nil {
		return 0, e
	}
	var lines []string
	for i := start; i <= end; i++ {
		id := fmt.Sprintf("BASIC5000_%04d", i)
		text := readingTexts[id]
		if text == "" {
			text = originalTexts[id]
		}
		if text == "" {
			continue
		}
		source := filepath.Join(audioDir, id+".wav")
		if _, err := os.Stat(source); err != nil {
			continue
		}
		dst := filepath.Join(out, "wavs", id+".wav")
		if err := os.Link(source, dst); err != nil {
			data, readErr := os.ReadFile(source)
			if readErr != nil {
				return 0, readErr
			}
			if err = os.WriteFile(dst, data, 0644); err != nil {
				return 0, err
			}
		}
		lines = append(lines, id+"|"+text+"|"+readingKanas[id]+"\n")
	}
	if len(lines) == 0 {
		return 0, fmt.Errorf("no sentences collected")
	}
	if e = os.WriteFile(filepath.Join(out, "metadata.csv"), []byte(strings.Join(lines, "")), 0644); e != nil {
		return 0, e
	}
	return len(lines), nil
}
func main() {
	root := flag.String("root", "", "extracted corpus root")
	out := flag.String("out", "", "new output directory under out/")
	start := flag.Int("start", 1, "first sentence")
	end := flag.Int("end", 600, "last sentence")
	audioDir := flag.String("audio-dir", "", "explicit BASIC5000 WAV directory")
	flag.Parse()
	if *root == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "--root and --out are required")
		os.Exit(2)
	}
	count, e := run(*root, *out, *audioDir, *start, *end)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	fmt.Printf("wrote %d sentences\n", count)
}
