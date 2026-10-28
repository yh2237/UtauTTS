package refspeech

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yh2237/UtauIDX/refmatch"
)

func TestListAndDeleteIndexes(t *testing.T) {
	dir := t.TempDir()
	path := writeIndex(t, dir, "sample.idx", refmatch.Version(), FeatureModel)
	index, err := refmatch.LoadIndex(path)
	if err != nil {
		t.Fatal(err)
	}
	index.Source = filepath.Join(dir, "voice", "bank")
	index.CacheKey = "sample"
	if err := index.Save(path); err != nil {
		t.Fatal(err)
	}
	modified := time.UnixMilli(1712345678000)
	if err := os.Chtimes(path, modified, modified); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.idx"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	indexes, err := ListIndexes(dir)
	if err != nil || len(indexes) != 1 {
		t.Fatalf("indexes=%+v err=%v", indexes, err)
	}
	got := indexes[0]
	if got.Name != "sample.idx" || got.Path != path || got.Voicebank != "bank" || got.Model != FeatureModel || got.Version != refmatch.Version() || got.SizeBytes != info.Size() || got.ModifiedMS != info.ModTime().UnixMilli() {
		t.Fatalf("unexpected index: %+v", got)
	}
	service := Shared(Models{}, dir)
	service.indexes["test"] = &indexEntry{index: index, status: IndexStatus{State: IndexReady}}
	deleted, err := DeleteIndex(dir, got.Name)
	if err != nil || !deleted {
		t.Fatalf("deleted=%v err=%v", deleted, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("index remains: %v", err)
	}
	if len(service.indexes) != 0 {
		t.Fatal("loaded index was retained")
	}
	if deleted, err := DeleteIndex(dir, got.Name); err != nil || deleted {
		t.Fatalf("missing: deleted=%v err=%v", deleted, err)
	}
}

func TestDeleteIndexRejectsPaths(t *testing.T) {
	for _, name := range []string{"", "../outside.idx", `..\outside.idx`, "C:outside.idx", "notes.txt", ".idx/other.idx"} {
		if _, err := DeleteIndex(t.TempDir(), name); err == nil {
			t.Errorf("accepted %q", name)
		}
	}
}

func TestListIndexesMissingDirectory(t *testing.T) {
	indexes, err := ListIndexes(filepath.Join(t.TempDir(), "missing"))
	if err != nil || indexes == nil || len(indexes) != 0 {
		t.Fatalf("indexes=%v err=%v", indexes, err)
	}
}

func TestPrepareStatusDoesNotWaitForBuild(t *testing.T) {
	service := Shared(Models{}, t.TempDir())
	root := t.TempDir()
	service.buildMu.Lock()
	service.engineMu.Lock()
	service.worldMu.Lock()
	status := service.PrepareStatus(root, nil, "")
	entry := service.entry(root, nil, "")
	service.worldMu.Unlock()
	service.engineMu.Unlock()
	service.buildMu.Unlock()
	if status.State != IndexBuilding {
		t.Fatalf("status=%+v", status)
	}
	select {
	case <-entry.done:
	case <-time.After(5 * time.Second):
		t.Fatal("background build did not finish")
	}
}

func TestMatchesSourceCompatibility(t *testing.T) {
	root := filepath.Join(t.TempDir(), "bank")
	for _, header := range []refmatch.Header{{Source: "key"}, {Source: root, CacheKey: "key"}} {
		if !matchesSource(&refmatch.Index{Header: header}, root, "key") {
			t.Fatalf("did not match %+v", header)
		}
	}
	if matchesSource(&refmatch.Index{Header: refmatch.Header{Source: root, CacheKey: "old"}}, root, "key") {
		t.Fatal("matched stale index")
	}
}
