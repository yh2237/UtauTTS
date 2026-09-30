package otofile

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"strconv"
	"strings"
)

type Entry struct {
	Filename     string
	Alias        string
	Offset       float64
	Fixed        float64
	Blank        float64
	Preutterance float64
	Overlap      float64
	Line         int
}

type Diagnostic struct {
	Line    int
	Message string
}

// Scanは復号済みテキストを読み、相対パスをbaseDir基準で解決する。ファイルの存在は検査しない。
// 正常行はvisitへ渡し、不正行は診断として残す。読込失敗や行長超過はエラーになる。
func Scan(reader io.Reader, baseDir string, visit func(Entry)) ([]Diagnostic, error) {
	if visit == nil {
		return nil, fmt.Errorf("oto visitor is required")
	}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	var diagnostics []Diagnostic
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		entry, err := parseLine(line, baseDir)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{Line: lineNumber, Message: err.Error()})
			continue
		}
		entry.Line = lineNumber
		visit(entry)
	}
	return diagnostics, scanner.Err()
}

func parseLine(line, baseDir string) (Entry, error) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return Entry{}, fmt.Errorf("missing '='")
	}
	filename := strings.TrimSpace(parts[0])
	if filename == "" {
		return Entry{}, fmt.Errorf("empty filename")
	}
	fields := strings.Split(parts[1], ",")
	if len(fields) < 6 {
		return Entry{}, fmt.Errorf("expected alias and 5 parameters, got %d fields", len(fields))
	}
	alias := strings.TrimSpace(fields[0])
	if alias == "" {
		alias = strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	}
	var values [5]float64
	for i := range values {
		value := strings.TrimSpace(fields[i+1])
		if value == "" {
			continue
		}
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return Entry{}, fmt.Errorf("invalid parameter %d %q", i+1, value)
		}
		if math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return Entry{}, fmt.Errorf("parameter %d %q must be finite", i+1, value)
		}
		values[i] = parsed
	}
	fullPath := filename
	if !filepath.IsAbs(filename) {
		fullPath = filepath.Join(baseDir, filepath.FromSlash(strings.ReplaceAll(filename, "\\", "/")))
	}
	return Entry{Filename: filepath.Clean(fullPath), Alias: alias,
		Offset: values[0], Fixed: values[1], Blank: values[2], Preutterance: values[3], Overlap: values[4]}, nil
}
