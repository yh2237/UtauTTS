package voicebank

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadARPAsingDictionary(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "oto.ini"), []byte("a.wav=hh ah,0,0,0,0,0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	data := []byte("entries:\n  - grapheme: Hello\n    phonemes: [hh, ah, l, ow]\n")
	if err := os.WriteFile(filepath.Join(root, "arpasing.yaml"), data, 0644); err != nil {
		t.Fatal(err)
	}
	bank, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if bank.ARPAsing["hello"] != "hh ah l ow" {
		t.Fatalf("ARPAsing = %#v", bank.ARPAsing)
	}
}
