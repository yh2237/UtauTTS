package synth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"utautts/internal/audio"
	"utautts/internal/plan"
	"utautts/internal/tts"
)

func TestResultAndExportContainSharedArtifacts(t *testing.T) {
	result, err := NewResult(&tts.Result{
		Audio:           &audio.PCM{SampleRate: 1000, Channels: 1, Data: make([]int16, 160)},
		Plan:            &plan.Plan{Reading: "あ", Units: []plan.Unit{{Position: 0, Role: "mora", Mora: "あ"}}},
		MoraDurationsMS: []float64{100},
	}, "waveform")
	if err != nil {
		t.Fatal(err)
	}
	if result.RendererID != "waveform" || result.DurationMS != 160 || !strings.Contains(result.Lab, " a\n") {
		t.Fatalf("unexpected result: %#v", result)
	}
	wavPath := filepath.Join(t.TempDir(), "sample.wav")
	if err := WriteFiles(wavPath, result, ExportOptions{Text: "あ", WriteText: true, WriteLab: true}); err != nil {
		t.Fatal(err)
	}
	for _, extension := range []string{".wav", ".txt", ".lab"} {
		if _, err := os.Stat(strings.TrimSuffix(wavPath, ".wav") + extension); err != nil {
			t.Fatalf("missing %s: %v", extension, err)
		}
	}
}
