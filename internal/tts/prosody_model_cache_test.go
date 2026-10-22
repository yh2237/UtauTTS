package tts

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReplacedProsodyModelDropsCachedPredictions(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "models", "frame-intonation-tcn-v12.json"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "model.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadProsodyModelCached(nil, path); err != nil {
		t.Fatal(err)
	}
	key := prosodyComputationKey{modelPath: path, text: "x"}
	synthesisCache.Lock()
	synthesisCache.computations[key] = prosodyComputation{}
	synthesisCache.computationOrder = append(synthesisCache.computationOrder, key)
	synthesisCache.Unlock()
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	if _, err := loadProsodyModelCached(nil, path); err != nil {
		t.Fatal(err)
	}
	synthesisCache.RLock()
	_, stale := synthesisCache.computations[key]
	synthesisCache.RUnlock()
	if stale {
		t.Fatal("predictions from the replaced model are still cached")
	}
}
