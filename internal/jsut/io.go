package jsut

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// ReadAlignmentsはjsut-join-datasetのJSONLを読む
// 複数の準備済みファイルを扱えるようJSON配列も受け付ける
func ReadAlignments(path string) ([]Alignment, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read alignment %s: %w", path, err)
	}
	trimmed := bytes.TrimSpace(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}))
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("alignment %s is empty", path)
	}
	if trimmed[0] == '[' {
		var records []Alignment
		if err := json.Unmarshal(trimmed, &records); err != nil {
			return nil, fmt.Errorf("decode alignment %s: %w", path, err)
		}
		return records, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read alignment %s: %w", path, err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 32*1024*1024)
	result := make([]Alignment, 0)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := bytes.TrimSpace(bytes.TrimPrefix(scanner.Bytes(), []byte{0xef, 0xbb, 0xbf}))
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		var record Alignment
		if err := json.Unmarshal(line, &record); err != nil {
			return nil, fmt.Errorf("decode alignment %s line %d: %w", path, lineNumber, err)
		}
		result = append(result, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read alignment %s: %w", path, err)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("alignment %s has no records", path)
	}
	return result, nil
}
