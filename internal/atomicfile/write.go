// atomicfileパッケージは置換完了まで既存ファイルを保持する。
package atomicfile

import (
	"io"
	"os"
	"path/filepath"
)

func Write(path string, write func(io.Writer) error) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".utautts-write-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	mode := os.FileMode(0644)
	if old, err := os.Stat(path); err == nil {
		mode = old.Mode().Perm()
	}
	if err = file.Chmod(mode); err != nil {
		return err
	}
	if err = write(file); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func WriteFile(path string, data []byte) error {
	return Write(path, func(w io.Writer) error { _, err := w.Write(data); return err })
}
