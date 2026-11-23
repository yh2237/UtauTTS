package refspeech

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yh2237/UtauIDX/encoder"
)

// TestTrimIndexCacheは、上限を超えた索引が古い順に消えることを確かめる。
func TestTrimIndexCache(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().Add(-time.Hour)
	var paths []string
	for i := 0; i < indexCacheMaxFiles+3; i++ {
		path := writeIndex(t, dir, fmt.Sprintf("index-%02d.utsp", i), encoder.IDWavLMBasePlusL6)
		stamp := base.Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	count, _ := trimIndexCache(dir)
	if count != 3 {
		t.Fatalf("trimmed %d indexes, want 3", count)
	}
	remaining, err := filepath.Glob(filepath.Join(dir, "*.utsp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != indexCacheMaxFiles {
		t.Fatalf("%d indexes remain, want %d", len(remaining), indexCacheMaxFiles)
	}
	// 最後に使ったのが最も新しいものが残る。
	if _, err := os.Stat(paths[len(paths)-1]); err != nil {
		t.Errorf("the most recently used index should remain: %v", err)
	}
	if _, err := os.Stat(paths[0]); !os.IsNotExist(err) {
		t.Error("the least recently used index should be removed")
	}
}

// TestTrimIndexCacheKeepsSmallCacheは、上限以内なら何も消さないことを確かめる。
func TestTrimIndexCacheKeepsSmallCache(t *testing.T) {
	dir := t.TempDir()
	path := writeIndex(t, dir, "only.utsp", encoder.IDWavLMBasePlusL6)
	if count, _ := trimIndexCache(dir); count != 0 {
		t.Fatalf("trimmed %d indexes from a small cache", count)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the only index should remain: %v", err)
	}
}

// TestTrimPartsは、部分キャッシュが上限を超えたら古い順に消えることを確かめる。
func TestTrimParts(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().Add(-time.Hour)
	var paths []string
	for i, name := range []string{"a.world", "b.chan", "c.part", "d.world"} {
		path := filepath.Join(dir, fmt.Sprintf("%02x", i), name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, make([]byte, 100), 0o644); err != nil {
			t.Fatal(err)
		}
		stamp := base.Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	other := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(other, make([]byte, 1000), 0o644); err != nil {
		t.Fatal(err)
	}
	count, bytes := trimParts(dir, 250)
	if count != 2 || bytes != 200 {
		t.Fatalf("trimmed %d parts (%d bytes), want 2 (200 bytes)", count, bytes)
	}
	for i, path := range paths {
		_, err := os.Stat(path)
		if removed := os.IsNotExist(err); removed != (i < 2) {
			t.Errorf("%s removed = %v", filepath.Base(path), removed)
		}
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("a file that is not a part was removed: %v", err)
	}
	if count, _ := trimParts(dir, 250); count != 0 {
		t.Fatalf("trimmed %d parts under the limit", count)
	}
}
