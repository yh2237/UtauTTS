package refspeech

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenReportsBrokenModel(t *testing.T) {
	dir := t.TempDir()
	weights := filepath.Join(dir, "kokoro-v1_0.pth")
	if err := os.WriteFile(weights, []byte("not a torch file"), 0o644); err != nil {
		t.Fatal(err)
	}
	engine, err := Open(Models{
		KokoroWeights: weights,
		KokoroConfig:  filepath.Join(dir, "missing.json"),
		KokoroVoice:   filepath.Join(dir, "missing.pt"),
		HuBERT:        filepath.Join(dir, "missing.safetensors"),
	})
	if err == nil {
		t.Fatal("Open accepted a broken model")
	}
	if engine != nil {
		t.Fatalf("Open returned an engine with the error: %v", err)
	}
}
