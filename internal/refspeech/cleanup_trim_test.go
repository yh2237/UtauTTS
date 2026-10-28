package refspeech

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yh2237/UtauIDX/refmatch"
)

// TestTrimIndexCacheは、上限を超えた索引が古い順に消えることを確かめる。
func TestTrimIndexCache(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().Add(-time.Hour)
	var paths []string
	for i := 0; i < indexCacheMaxFiles+3; i++ {
		path := writeIndex(t, dir, fmt.Sprintf("index-%02d.idx", i), refmatch.Version(), FeatureModel)
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
	remaining, err := filepath.Glob(filepath.Join(dir, "*.idx"))
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
	path := writeIndex(t, dir, "only.idx", refmatch.Version(), FeatureModel)
	if count, _ := trimIndexCache(dir); count != 0 {
		t.Fatalf("trimmed %d indexes from a small cache", count)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the only index should remain: %v", err)
	}
}
