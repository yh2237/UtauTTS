package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProsodyFeatures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "features.json")
	data := []byte(`{"version":1,"cases":[{"id":"sample","features":[{"accent_high":1},{"accent_high":0,"word_end":1}]}]}`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	frames, err := loadProsodyFeatures(path, "sample", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 2 || frames[0]["accent_high"] != 1 || frames[1]["word_end"] != 1 {
		t.Fatalf("unexpected frames: %#v", frames)
	}
}

func TestLoadDictionaryAndMoraDurations(t *testing.T) {
	directory := t.TempDir()
	dictionaryPath := filepath.Join(directory, "dictionary.json")
	if err := os.WriteFile(dictionaryPath, []byte(`[{"surface":"v8","reading":"ぶいはち"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := loadDictionary(dictionaryPath)
	if err != nil || len(entries) != 1 || entries[0].Reading != "ぶいはち" {
		t.Fatalf("dictionary = %#v, %v", entries, err)
	}
	durationsPath := filepath.Join(directory, "durations.json")
	if err := os.WriteFile(durationsPath, []byte(`{"mora_durations_ms":[100,180]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	durations, err := loadMoraDurations(durationsPath)
	if err != nil || len(durations) != 2 || durations[1] != 180 {
		t.Fatalf("durations = %#v, %v", durations, err)
	}
}
