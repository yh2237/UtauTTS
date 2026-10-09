package main

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
	"utautts/cmd/tools/internal/toolutil"
)

func decodedName(f *zip.File) string {
	if f.Flags&0x800 != 0 {
		return f.Name
	}
	// UTF-8フラグが無い名前は元のバイト列のままなので、Shift_JISとして復号する。
	name, _, err := transform.String(japanese.ShiftJIS.NewDecoder(), f.Name)
	if err != nil || strings.ContainsRune(name, '\ufffd') {
		return f.Name
	}
	return name
}

func build(zipPath, out string) (string, int, error) {
	if err := toolutil.RequireUnderOut(out, "output", false); err != nil {
		return "", 0, err
	}
	if _, err := os.Stat(out); err == nil {
		return "", 0, fmt.Errorf("refusing to overwrite %s", out)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", 0, err
	}
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", 0, err
	}
	defer r.Close()
	type entry struct {
		file *zip.File
		name string
	}
	var entries []entry
	bankRel := ""
	bestDepth := int(^uint(0) >> 1)
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := strings.ReplaceAll(decodedName(f), "\\", "/")
		entries = append(entries, entry{f, name})
		if strings.EqualFold(path.Base(name), "oto.ini") && strings.Count(name, "/") < bestDepth {
			bestDepth = strings.Count(name, "/")
			bankRel = path.Dir(name)
			if bankRel == "." {
				bankRel = ""
			}
		}
	}
	bankName := path.Base(bankRel)
	if bankRel == "" {
		bankName = strings.TrimSuffix(filepath.Base(zipPath), filepath.Ext(zipPath))
	}
	if bankName == "" || bankName == "." || bankName == ".." {
		return "", 0, fmt.Errorf("invalid voicebank name")
	}
	files := map[string]int64{}
	for _, item := range entries {
		rel := item.name
		if bankRel != "" {
			if !strings.HasPrefix(rel, bankRel+"/") {
				continue
			}
			rel = strings.TrimPrefix(rel, bankRel+"/")
		}
		clean := path.Clean(rel)
		if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) || strings.Contains(clean, ":") {
			return "", 0, fmt.Errorf("invalid ZIP path: %s", item.name)
		}
		destination := filepath.Join(out, bankName, filepath.FromSlash(clean))
		if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			return "", 0, err
		}
		src, err := item.file.Open()
		if err != nil {
			return "", 0, err
		}
		dst, err := toolutil.CreateExclusive(destination)
		if err != nil {
			src.Close()
			return "", 0, err
		}
		_, copyErr := io.Copy(dst, src)
		closeErr := dst.Close()
		src.Close()
		if copyErr != nil {
			return "", 0, copyErr
		}
		if closeErr != nil {
			return "", 0, closeErr
		}
		info, err := os.Stat(destination)
		if err != nil {
			return "", 0, err
		}
		files[bankName+"/"+clean] = info.Size()
	}
	data, err := json.Marshal(map[string]any{"files": files})
	if err != nil {
		return "", 0, err
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		return "", 0, err
	}
	manifest, err := toolutil.CreateExclusive(filepath.Join(out, "manifest.json"))
	if err != nil {
		return "", 0, err
	}
	_, err = manifest.Write(data)
	closeErr := manifest.Close()
	if err != nil {
		return "", 0, err
	}
	return bankName, len(files), closeErr
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/tools/build-voice ZIP out/VOICE_DIR")
		os.Exit(2)
	}
	name, count, err := build(os.Args[1], os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("packaged voice bank %s: %d files\n", name, count)
}
