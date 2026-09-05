package plugin

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxPackageBytes = 512 << 20

// InstallPackageは公開前にパッケージを検証する。既存のインストール先は変更しない。
func InstallPackage(archive, destination string, supportsBackend func(string) bool) (Renderer, error) {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return Renderer{}, err
	}
	defer reader.Close()
	if len(reader.File) > 4096 {
		return Renderer{}, fmt.Errorf("package has too many files")
	}
	if err := os.MkdirAll(destination, 0755); err != nil {
		return Renderer{}, err
	}
	stage, err := os.MkdirTemp(destination, ".install-")
	if err != nil {
		return Renderer{}, err
	}
	defer os.RemoveAll(stage)
	var total int64
	seen := map[string]bool{}
	for _, entry := range reader.File {
		if entry.Mode()&(os.ModeSymlink|os.ModeDevice|os.ModeNamedPipe|os.ModeSocket) != 0 {
			return Renderer{}, fmt.Errorf("unsupported package entry %q", entry.Name)
		}
		path, err := packagePath(stage, entry.Name)
		if err != nil {
			return Renderer{}, err
		}
		key := strings.ToLower(path)
		if seen[key] {
			return Renderer{}, fmt.Errorf("duplicate package path %q", entry.Name)
		}
		seen[key] = true
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(path, 0755); err != nil {
				return Renderer{}, err
			}
			continue
		}
		if entry.UncompressedSize64 > maxPackageBytes || total+int64(entry.UncompressedSize64) > maxPackageBytes {
			return Renderer{}, fmt.Errorf("package exceeds 512 MiB")
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return Renderer{}, err
		}
		input, err := entry.Open()
		if err != nil {
			return Renderer{}, err
		}
		mode := os.FileMode(0644)
		if entry.Mode().Perm()&0111 != 0 {
			mode = 0755
		}
		output, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
		if err != nil {
			input.Close()
			return Renderer{}, err
		}
		n, copyErr := io.Copy(output, io.LimitReader(input, maxPackageBytes-total+1))
		input.Close()
		closeErr := output.Close()
		total += n
		if copyErr != nil {
			return Renderer{}, copyErr
		}
		if closeErr != nil {
			return Renderer{}, closeErr
		}
		if total > maxPackageBytes {
			return Renderer{}, fmt.Errorf("package exceeds 512 MiB")
		}
	}
	items, err := DiscoverRenderers([]string{stage}, supportsBackend)
	if err != nil {
		return Renderer{}, err
	}
	if len(items) != 1 || items[0].ManifestVersion != 2 {
		return Renderer{}, fmt.Errorf("ZIP must contain exactly one v2 renderer manifest")
	}
	item := items[0]
	for _, builtin := range BuiltinRenderers() {
		if builtin.ID == item.ID {
			return Renderer{}, fmt.Errorf("built-in ID %q is reserved", item.ID)
		}
	}
	installed, _ := DiscoverRenderers([]string{destination}, supportsBackend)
	for _, existing := range installed {
		if strings.EqualFold(existing.ID, item.ID) {
			return Renderer{}, fmt.Errorf("renderer %q is already installed", item.ID)
		}
	}
	for name, path := range item.Assets {
		if info, err := os.Stat(item.Asset(name)); err != nil || !info.Mode().IsRegular() {
			return Renderer{}, fmt.Errorf("required asset %s is unavailable: %s", name, path)
		}
	}
	target := filepath.Join(destination, item.ID)
	if _, err := os.Lstat(target); err == nil || !os.IsNotExist(err) {
		return Renderer{}, fmt.Errorf("package destination already exists")
	}
	if err := os.Rename(item.Directory, target); err != nil {
		return Renderer{}, err
	}
	data, err := os.ReadFile(filepath.Join(target, "plugin.json"))
	if err != nil {
		return Renderer{}, err
	}
	item, err = decodeRenderer(data, target)
	item.Directory = target
	return item, err
}
