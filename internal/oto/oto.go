package oto

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"utautts/internal/otofile"
)

type Entry struct {
	Filename     string
	Alias        string
	Offset       float64
	Fixed        float64
	Blank        float64
	Preutterance float64
	Overlap      float64
	OtoPath      string
	Line         int
	SourceGroup  string
}

type Diagnostic struct {
	Line    int
	Message string
}

type Ini struct {
	Path        string
	Encoding    string
	Entries     map[string][]Entry
	Diagnostics []Diagnostic
}

func ReadIni(otoPath string) (*Ini, error) {
	data, err := os.ReadFile(otoPath)
	if err != nil {
		return nil, err
	}

	text, encoding, err := Decode(data)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", otoPath, err)
	}
	absPath, err := filepath.Abs(otoPath)
	if err != nil {
		return nil, err
	}

	return parseText(text, encoding, absPath, filepath.Dir(absPath))
}

// sourceは出典表示用で、baseDirは録音パスの基準。どちらも作業ディレクトリから補完しない。
func ParseBytes(data []byte, source, baseDir string) (*Ini, error) {
	text, encoding, err := Decode(data)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", source, err)
	}
	return parseText(text, encoding, source, baseDir)
}

func parseText(text, encoding, source, baseDir string) (*Ini, error) {
	result := &Ini{Path: source, Encoding: encoding, Entries: map[string][]Entry{}}
	diagnostics, err := otofile.Scan(strings.NewReader(text), baseDir, func(record otofile.Entry) {
		entry := Entry{Filename: record.Filename, Alias: record.Alias, Offset: record.Offset,
			Fixed: record.Fixed, Blank: record.Blank, Preutterance: record.Preutterance,
			Overlap: record.Overlap, OtoPath: source, Line: record.Line}
		result.Entries[entry.Alias] = append(result.Entries[entry.Alias], entry)
	})
	if err != nil {
		return nil, err
	}
	for _, diagnostic := range diagnostics {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Line: diagnostic.Line, Message: diagnostic.Message})
	}
	return result, nil
}
