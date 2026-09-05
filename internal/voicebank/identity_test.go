package voicebank

import (
	"path/filepath"
	"testing"
)

func TestStableIDsAndLegacyAmbiguity(t *testing.T) {
	root := t.TempDir()
	a, b := StableID(root, filepath.Join(root, "A", "Voice")), StableID(root, filepath.Join(root, "B", "Voice"))
	if a == b {
		t.Fatal("ID collision")
	}
	items := map[string]int{a: 1, b: 2}
	if _, ok := ResolveLegacyID(items, "Voice"); ok {
		t.Fatal("ambiguous legacy ID resolved")
	}
	delete(items, b)
	if got, ok := ResolveLegacyID(items, "Voice"); !ok || got != 1 {
		t.Fatal("legacy ID no longer works")
	}
}
