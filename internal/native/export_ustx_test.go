package native

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/yh2237/UtauTTS/internal/openutau"
)

func TestExportUstxKeepsSavedPitchWithoutModelResources(t *testing.T) {
	engine := newTestEngine(t, Config{VoiceDir: t.TempDir()})
	outputPath := filepath.Join(t.TempDir(), "out.ustx")
	request, err := json.Marshal(map[string]any{
		"output_path": outputPath,
		"project": map[string]any{
			"format": "utautts-project", "format_version": 8,
			"utterances": []any{map[string]any{
				"text": "あ", "voicebank_id": "bank", "tone": "C4",
				"model_id": "unavailable-model", "apply_pitch": true,
				"automatic_frame_ms": 10, "automatic_frame_pitch": []float64{0, 100, 0},
				"manual_pitch_edited": true, "pitch_frames": []float64{0, 20, 0},
				"analysis_cache": map[string]any{
					"reading": "ア", "morae": []any{map[string]any{"mora": "あ"}},
				},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Call("exportUstx", request); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	var got openutau.USTXProject
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	points := got.VoiceParts[0].Notes[0].Pitch.Data
	if len(points) < 3 || points[1].X != 10 || points[1].Y != 12 {
		t.Fatalf("saved pitch and frame edit were not exported: %+v", points)
	}
}
