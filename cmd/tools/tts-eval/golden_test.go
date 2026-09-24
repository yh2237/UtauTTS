package main

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"

	"utautts/internal/audio"
	"utautts/internal/plan"
	"utautts/internal/plugin"
	"utautts/internal/synth"
)

// goldenSelectionDigestは固定した音源・読みに対する選択計画の指紋。
// 原音選択や時間配分を意図的に変えた場合だけ更新する。
const goldenSelectionDigest = "7644099acab9a37dfa93826696f2abe0bc25b293eab3bf21eadaa5e020205fdd"

// TestSelectionPlanGoldenは選択計画の決定性と内容の変化を検出する。
// 音源はテスト内で生成するため、配布アセットやruntimeなしで実行できる。
func TestSelectionPlanGolden(t *testing.T) {
	bankDir := filepath.Join(t.TempDir(), "bank")
	if err := os.Mkdir(bankDir, 0755); err != nil {
		t.Fatal(err)
	}
	samples := make([]int16, 8000)
	for index := range samples {
		samples[index] = int16(2000 * math.Sin(2*math.Pi*220*float64(index)/16000))
	}
	if err := audio.WriteWav(filepath.Join(bankDir, "a.wav"), &audio.PCM{SampleRate: 16000, Channels: 1, Data: samples}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bankDir, "oto.ini"), []byte("a.wav=か,0,40,0,60,30\na.wav=さ,0,50,0,70,35\n"), 0644); err != nil {
		t.Fatal(err)
	}
	catalog := &plugin.Catalog{Renderers: []plugin.Renderer{{
		ManifestVersion: 2, Kind: "synthesis-engine", ID: "waveform", DisplayName: "waveform",
		Contract: "unit-renderer", Provider: "waveform", ProviderVersion: "1",
	}}}
	service := synth.NewService(catalog, "waveform", "", "", "", nil)
	request := synth.Request{
		Reading: "かさ", VoicebankPath: bankDir, Tone: "C4", Renderer: "waveform",
		MoraDurationMS: synth.DefaultMoraDurationMS, PauseDurationMS: plan.DefaultPauseDurationMS,
	}
	first, err := service.SynthesizeContext(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.SynthesizeContext(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	digest := normalizedSelectionDigest(first.Plan)
	if digest == "" {
		t.Fatal("empty selection plan digest")
	}
	if rerun := normalizedSelectionDigest(second.Plan); rerun != digest {
		t.Fatalf("selection plan digest is not deterministic: %s vs %s", digest, rerun)
	}
	if digest != goldenSelectionDigest {
		t.Fatalf("selection plan digest = %s, want %s (update goldenSelectionDigest if the change is intended)", digest, goldenSelectionDigest)
	}
}

// normalizedSelectionDigestは原音パスの絶対パス差を除いて指紋を取る。
func normalizedSelectionDigest(p *plan.Plan) string {
	if p == nil {
		return ""
	}
	clone := *p
	clone.Units = make([]plan.Unit, len(p.Units))
	copy(clone.Units, p.Units)
	for index := range clone.Units {
		clone.Units[index].Source = filepath.Base(clone.Units[index].Source)
	}
	return selectionPlanDigest(&clone)
}
