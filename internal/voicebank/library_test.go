package voicebank

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLibraryReloadsAndResolvesStableVoicebankIDs(t *testing.T) {
	root := t.TempDir()
	bank := filepath.Join(root, "bank")
	if err := os.Mkdir(bank, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bank, "oto.ini"), []byte("a.wav=あ,0,0,0,0,0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	library := NewLibrary(root)
	if err := library.Reload(); err != nil {
		t.Fatal(err)
	}
	items := library.List()
	if len(items) != 1 || items[0].ID != StableID(root, bank) {
		t.Fatalf("library items = %#v", items)
	}
	resolved, ok := library.Resolve("")
	if !ok || resolved.Path != bank {
		t.Fatalf("default resolution = %#v, %v", resolved, ok)
	}
}
