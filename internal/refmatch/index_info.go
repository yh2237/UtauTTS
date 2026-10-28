package refmatch

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
)

// IndexInfoは索引のヘッダとファイルの情報。フレームの配列は読み込まない。
type IndexInfo struct {
	Header
	Name       string
	Path       string
	SizeBytes  int64
	ModifiedMS int64
}

// ReadIndexInfoは索引のヘッダとファイルの情報だけを読む。
func ReadIndexInfo(path string) (IndexInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return IndexInfo{}, err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return IndexInfo{}, err
	}
	if !stat.Mode().IsRegular() {
		return IndexInfo{}, fmt.Errorf("not a regular index file: %s", path)
	}
	// ReadHeaderと同じ形式で、配列より前のヘッダだけを読む。
	header, err := readHeader(bufio.NewReader(file))
	if err != nil {
		return IndexInfo{}, err
	}
	return IndexInfo{Header: header, Name: filepath.Base(path), Path: path, SizeBytes: stat.Size(), ModifiedMS: stat.ModTime().UnixMilli()}, nil
}
