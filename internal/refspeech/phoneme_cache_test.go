package refspeech

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yh2237/UtauIDX/refmatch"
)

func TestPhonemeChangesInvalidateIndexCache(t *testing.T) {
	root := t.TempDir()
	recordings := []refmatch.Recording{{Path: filepath.Join(root, "a.wav")}}
	if err := os.WriteFile(recordings[0].Path, []byte("RIFF"), 0600); err != nil {
		t.Fatal(err)
	}
	oldID := indexID(root, recordings)
	_, oldKey, err := bankRecordings(root, recordings)
	if err != nil {
		t.Fatal(err)
	}
	recordings[0].Phoneme = "a"
	id := indexID(root, recordings)
	files, key, err := bankRecordings(root, recordings)
	if err != nil {
		t.Fatal(err)
	}
	if id == oldID || key == oldKey {
		t.Fatal("phoneme label did not invalidate index cache")
	}
	if files[0].Phoneme != "a" {
		t.Fatal("phoneme label dropped")
	}
	recordings[0].Phoneme = "ka"
	_, nextKey, err := bankRecordings(root, recordings)
	if err != nil {
		t.Fatal(err)
	}
	if nextKey == key || indexID(root, recordings) == id {
		t.Fatal("changed label did not invalidate index cache")
	}
}
