package plugin

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testV2 = `{"manifest_version":2,"kind":"renderer","id":"example.wave","display_name":"Example","version":"1.0","backend":"waveform","protocol_version":1}`

func packageZIP(t *testing.T, files map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "plugin.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(file)
	for name, data := range files {
		entry, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestInstallV2PackageAndRejectDuplicate(t *testing.T) {
	dir := t.TempDir()
	archive := packageZIP(t, map[string]string{"example/plugin.json": testV2})
	item, err := InstallPackage(archive, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if item.ID != "example.wave" || !item.Capabilities.FramePitch {
		t.Fatalf("renderer=%+v", item)
	}
	if _, err := InstallPackage(archive, dir, nil); err == nil {
		t.Fatal("existing installation replaced without upgrade")
	}
	data, _ := os.ReadFile(filepath.Join(dir, item.ID, "plugin.json"))
	if string(data) != testV2 {
		t.Fatal("existing manifest changed")
	}
}

func TestPackageRejectsEscapeAndUnknownFields(t *testing.T) {
	for _, files := range []map[string]string{
		{"../escape": "bad", "plugin.json": testV2},
		{"C:/escape": "bad", "plugin.json": testV2},
		{"plugin.json": strings.Replace(testV2, `"version":"1.0"`, `"version":"1.0","backned":"waveform"`, 1)},
		{"plugin.json": strings.Replace(testV2, `"backend":"waveform"`, `"backend":"utau-external-resampler"`, 1)},
	} {
		dir := t.TempDir()
		if _, err := InstallPackage(packageZIP(t, files), dir, nil); err == nil {
			t.Fatal("invalid package accepted")
		}
		entries, _ := os.ReadDir(dir)
		if len(entries) != 0 {
			t.Fatalf("partial package remains: %v", entries)
		}
	}
}

func TestV2RejectsHashMismatchAndEscapingAsset(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "engine.dll"), []byte("test"), 0644)
	for _, path := range []string{"engine.dll", "../engine.dll"} {
		doc := fmt.Sprintf(`{"manifest_version":2,"kind":"renderer","id":"example.world","display_name":"Example","version":"1","backend":"utautts-world-phrase","protocol_version":1,"platforms":{"any":{"world_engine":{"path":%q,"sha256":%q}}}}`, path, strings.Repeat("0", 64))
		if _, err := decodeRenderer([]byte(doc), dir); err == nil {
			t.Fatal("invalid asset accepted")
		}
	}
}

func TestBuiltinCatalogDoesNotRequireManifestFiles(t *testing.T) {
	items := BuiltinRenderers()
	found := false
	for _, item := range items {
		if item.ID == "utautts-world-phrase" {
			found = true
			if item.Asset("world_engine") == "" || !item.Capabilities.FramePitch {
				t.Fatal("incomplete builtin")
			}
		}
	}
	if !found {
		t.Fatal("default renderer absent")
	}
}
