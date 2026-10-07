package main

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

func TestBuild(t *testing.T) {
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(filepath.Join("..", "..", "..")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(previous) })
	root := filepath.Join("out", "build-voice-test")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	archive := filepath.Join(root, "voice.zip")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	for name, body := range map[string]string{"wrapper/bank/oto.ini": "a", "wrapper/bank/a.wav": "wav", "unrelated/other.txt": "skip"} {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "packaged")
	name, count, err := build(archive, out)
	if err != nil {
		t.Fatal(err)
	}
	if name != "bank" || count != 2 {
		t.Fatalf("name=%q count=%d", name, count)
	}
	data, err := os.ReadFile(filepath.Join(out, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Files map[string]int64 `json:"files"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Files["bank/oto.ini"] != 1 || manifest.Files["bank/a.wav"] != 3 || len(manifest.Files) != 2 {
		t.Fatalf("manifest: %+v", manifest)
	}
	if _, _, err := build(archive, out); err == nil {
		t.Fatal("expected overwrite refusal")
	}
}

func TestDecodedNameCP932(t *testing.T) {
	encoded, _, err := transform.String(japanese.ShiftJIS.NewEncoder(), "音源/oto.ini")
	if err != nil {
		t.Fatal(err)
	}
	f := &zip.File{FileHeader: zip.FileHeader{Name: encoded, NonUTF8: true}}
	if got := decodedName(f); got != "音源/oto.ini" {
		t.Fatalf("decoded name = %q", got)
	}
	f.Flags = 0x800
	f.Name = "音源/oto.ini"
	if got := decodedName(f); got != "音源/oto.ini" {
		t.Fatalf("UTF-8 name = %q", got)
	}
}
