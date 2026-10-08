package prosody

import (
	"os"
	"path/filepath"
	"testing"
)

// 同梱の統合韻律モデルJSONが、基準モデルとF0・エネルギーヘッドを合わせて読めることを確認する。
func TestBundledF0HeadModelLoads(t *testing.T) {
	path := filepath.Join("..", "..", "models", "speech-timing-target-ja-prosody-v1.json")
	model, err := LoadModel(path)
	if err != nil {
		t.Fatal(err)
	}
	if model.ID != "speech-timing-target-ja-prosody-v1" || model.Language != "ja" || model.DefaultPriority >= 120 {
		t.Fatalf("identity = %q %q %d", model.ID, model.Language, model.DefaultPriority)
	}
	if !model.HasFrameContour() || !model.RequiresExternalFeatures() {
		t.Fatal("base model features were not inherited")
	}
	head := model.F0Head
	if head == nil || !head.HasF0Head() || !head.HasEnergyHead() {
		t.Fatal("F0 head is missing")
	}
	if head.F0Scale() != 100 || len(head.Phones()) != 40 || head.Mels() != 80 {
		t.Fatalf("f0 scale %v phones %d mels %d", head.F0Scale(), len(head.Phones()), head.Mels())
	}
	if len(head.PosVocab()) != 11 || len(head.PosGroup1Vocab()) != 28 {
		t.Fatalf("pos vocab %d/%d", len(head.PosVocab()), len(head.PosGroup1Vocab()))
	}
}

func TestF0HeadModelRejectsMissingHead(t *testing.T) {
	directory := t.TempDir()
	base, err := os.ReadFile(filepath.Join("..", "..", "models", "frame-intonation-tcn-v10.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "base.json"), base, 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := []byte(`{"id":"broken","display_name":"Broken","base_model":"base.json","f0_head":""}`)
	path := filepath.Join(directory, "broken.json")
	if err := os.WriteFile(path, manifest, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadModel(path); err == nil {
		t.Fatal("manifest without F0 head weights must fail")
	}
}

// 既定の日本語抑揚モデルv11は、v10を0.65混ぜ、エネルギーを使わない設定で読める。
func TestBundledV11BlendsWithV10(t *testing.T) {
	model, err := LoadModel(filepath.Join("..", "..", "models", "intonation-ja-v11.json"))
	if err != nil {
		t.Fatal(err)
	}
	if model.F0Head == nil || model.F0HeadBaseBlend != 0.65 || model.F0HeadEnergy || model.DefaultPriority <= 120 {
		t.Fatalf("v11: head %v blend %v energy %v priority %d", model.F0Head != nil, model.F0HeadBaseBlend, model.F0HeadEnergy, model.DefaultPriority)
	}
}
