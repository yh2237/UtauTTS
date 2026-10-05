// Package toolutilはcmd/tools配下で共有する入出力ヘルパを提供する。
package toolutil

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maxJSONLLineBytes = 16 << 20

// ScanJSONLBytesはBOMと空行を除き、各行をhandleへ渡す。lineは次のScanまで有効。
func ScanJSONLBytes(data []byte, handle func(line []byte) error) error {
	scanner := bufio.NewScanner(bytes.NewReader(bytes.TrimPrefix(data, []byte("\ufeff"))))
	scanner.Buffer(make([]byte, 4096), maxJSONLLineBytes)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if err := handle(line); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// ScanJSONLはファイルを読み、各行をhandleへ渡す。
func ScanJSONL(path string, handle func(line []byte) error) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return ScanJSONLBytes(data, handle)
}

// CreateExclusiveは既存ファイルを上書きせずに新規作成する。
func CreateExclusive(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
}

// UnderOutはout/配下（out自身を含む）かを返す。
func UnderOut(path string) bool {
	clean := filepath.Clean(path)
	return clean == "out" || strings.HasPrefix(clean, "out"+string(filepath.Separator))
}

// UnderOutChildはout/の子（out自身は不可）かを返す。
func UnderOutChild(path string) bool {
	return strings.HasPrefix(filepath.Clean(path), "out"+string(filepath.Separator))
}

// RequireUnderOutはout/配下かを検証する。allowRootが真ならout自身も許す。
func RequireUnderOut(path, label string, allowRoot bool) error {
	valid := UnderOutChild(path)
	if allowRoot && !valid {
		valid = filepath.Clean(path) == "out"
	}
	if !valid {
		return fmt.Errorf("%s must be under out/", label)
	}
	return nil
}
