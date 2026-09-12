package voicebank

import (
	"path/filepath"
	"testing"
)

func TestStableIDs(t *testing.T) {
	root := t.TempDir()
	a, b := StableID(root, filepath.Join(root, "A", "Voice")), StableID(root, filepath.Join(root, "B", "Voice"))
	if a == b {
		t.Fatal("ID collision")
	}
}
