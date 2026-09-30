package main

import (
	"os"
	"path/filepath"
	"testing"
)

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
