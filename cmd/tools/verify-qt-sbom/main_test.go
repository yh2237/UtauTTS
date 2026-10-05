package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyAcceptsValidPackage(t *testing.T) {
	root := t.TempDir()
	sbomDir := writeSBOMPackage(t, root, validPackages())
	writeFile(t, filepath.Join(root, "Qt6Core.dll"), "qt")
	if err := verify(root, sbomDir, true); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyRejectsBundledFFmpeg(t *testing.T) {
	root := t.TempDir()
	sbomDir := writeSBOMPackage(t, root, validPackages())
	writeFile(t, filepath.Join(root, "Qt6Core.dll"), "qt")
	writeFile(t, filepath.Join(root, "avcodec-61.dll"), "ffmpeg")
	err := verify(root, sbomDir, true)
	if err == nil || !strings.Contains(err.Error(), "FFmpeg files must not be bundled") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyRejectsStaleFFmpegNotice(t *testing.T) {
	root := t.TempDir()
	sbomDir := writeSBOMPackage(t, root, validPackages())
	writeFile(t, filepath.Join(root, "Qt6Core.dll"), "qt")
	writeFile(t, filepath.Join(root, "licenses", "Qt", "LGPL-2.1.txt"), "stale")
	err := verify(root, sbomDir, true)
	if err == nil || !strings.Contains(err.Error(), "obsolete bundled-FFmpeg license file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyRejectsTamperedManifest(t *testing.T) {
	root := t.TempDir()
	sbomDir := writeSBOMPackage(t, root, validPackages())
	writeFile(t, filepath.Join(root, "Qt6Core.dll"), "qt")
	manifestPath := filepath.Join(root, "licenses", "Qt", "Qt-SBOM-MANIFEST.txt")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	tampered := strings.Replace(string(data), "SHA-256: ", "SHA-256: 0", 1)
	writeFile(t, manifestPath, tampered)
	err = verify(root, sbomDir, true)
	if err == nil || !strings.Contains(err.Error(), "hash is missing or incorrect") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyRejectsUnknownQtLibrary(t *testing.T) {
	root := t.TempDir()
	sbomDir := writeSBOMPackage(t, root, validPackages())
	writeFile(t, filepath.Join(root, "Qt6Widgetz.dll"), "qt")
	err := verify(root, sbomDir, true)
	if err == nil || !strings.Contains(err.Error(), "no matching SBOM module") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyRejectsMissingFFmpegPackage(t *testing.T) {
	root := t.TempDir()
	packages := validPackages()
	packages["qtmultimedia"] = []any{map[string]any{"name": "qtmultimedia"}}
	sbomDir := writeSBOMPackage(t, root, packages)
	writeFile(t, filepath.Join(root, "Qt6Core.dll"), "qt")
	err := verify(root, sbomDir, true)
	if err == nil || !strings.Contains(err.Error(), "exactly one FFmpeg package") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyRejectsMissingManifest(t *testing.T) {
	root := t.TempDir()
	sbomDir := writeSBOMPackage(t, root, validPackages())
	if err := os.Remove(filepath.Join(root, "licenses", "Qt", "Qt-SBOM-MANIFEST.txt")); err != nil {
		t.Fatal(err)
	}
	err := verify(root, sbomDir, true)
	if err == nil || !strings.Contains(err.Error(), "manifest is missing") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func validPackages() map[string][]any {
	return map[string][]any{
		"qtbase":        {map[string]any{"name": "qtbase"}},
		"qtdeclarative": {map[string]any{"name": "qtdeclarative"}},
		"qtmultimedia":  {map[string]any{"name": "qtmultimedia"}, map[string]any{"name": "FFmpeg"}},
	}
}

func writeSBOMPackage(t *testing.T, root string, packages map[string][]any) string {
	t.Helper()
	sbomDir := filepath.Join(root, "licenses", "Qt", "sbom")
	if err := os.MkdirAll(sbomDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var manifest strings.Builder
	for _, module := range sbomModules {
		name := module + "-6.0.0.spdx.json"
		object := map[string]any{
			"spdxVersion": "SPDX-2.3",
			"name":        module + "-6.0.0",
			"packages":    packages[module],
		}
		data, err := json.Marshal(object)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(sbomDir, name)
		writeFile(t, path, string(data))
		sum := sha256.Sum256(data)
		fmt.Fprintf(&manifest, "%s\nSHA-256: %s\n", name, strings.ToUpper(hex.EncodeToString(sum[:])))
	}
	writeFile(t, filepath.Join(root, "licenses", "Qt", "Qt-SBOM-MANIFEST.txt"), manifest.String())
	writeFile(t, filepath.Join(root, "licenses", "Qt", "FFmpeg-OPTIONAL.txt"), "optional")
	return sbomDir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
