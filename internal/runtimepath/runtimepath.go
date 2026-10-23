// Package runtimepathは、runtimeフォルダに置くライブラリと辞書を探す。
package runtimepath

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// LibraryNameは、OSに合わせた共有ライブラリのファイル名。
func LibraryName(base string) string {
	switch runtime.GOOS {
	case "windows":
		return base + ".dll"
	case "darwin":
		return base + ".dylib"
	}
	return base + ".so"
}

// Findは、明示されたパスか、実行ファイルの場所・作業フォルダの下のruntime/nameを返す。
func Find(explicit, name string, directory bool) (string, error) {
	if explicit != "" {
		if exists(explicit, directory) {
			return filepath.Abs(explicit)
		}
		return "", fmt.Errorf("%s not found: %s", name, explicit)
	}
	for _, root := range roots() {
		if candidate := filepath.Join(root, "runtime", name); exists(candidate, directory) {
			return filepath.Abs(candidate)
		}
	}
	return "", fmt.Errorf("%s not found; expected runtime/%s", name, name)
}

func exists(path string, directory bool) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir() == directory
}

func roots() []string {
	var result []string
	if executable, err := os.Executable(); err == nil {
		result = append(result, filepath.Dir(executable), filepath.Dir(filepath.Dir(executable)))
	}
	if current, err := os.Getwd(); err == nil {
		result = append(result, current)
	}
	return result
}
