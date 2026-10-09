package native

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// 外部資源なしのEngineを作り、ローカル環境に依存しない出力を検証する。
func exportEngine(t *testing.T) *Engine {
	t.Helper()
	engine, err := New(Config{
		VoiceDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("construct engine: %v", err)
	}
	return engine
}

func exportProject(t *testing.T, engine *Engine, project map[string]any) string {
	t.Helper()
	projectData, err := json.Marshal(project)
	if err != nil {
		t.Fatal(err)
	}
	outputPath := filepath.Join(t.TempDir(), "out.ustx")
	return exportProjectTo(t, engine, outputPath, projectData)
}

type exportRequest struct {
	OutputPath string          `json:"output_path"`
	Project    json.RawMessage `json:"project"`
}

func exportProjectTo(t *testing.T, engine *Engine, outputPath string, projectData []byte) string {
	t.Helper()
	request, err := json.Marshal(exportRequest{OutputPath: outputPath, Project: projectData})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.exportUstx(request); err != nil {
		t.Fatalf("exportUstx error: %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("exported USTX does not parse: %v", err)
	}
	return string(data)
}

func utteranceWithReading(text, reading string) map[string]any {
	morae := []map[string]any{}
	for index, mora := range strings.Split(reading, "") {
		if mora == "" {
			continue
		}
		entry := map[string]any{"position": index, "mora": mora}
		if mora == "、" || mora == "。" {
			entry["pause"] = true
		} else if mora == "ー" {
			// 母音は解析側で前のモーラから引き継ぐため、空でよい。
			entry["vowel"] = ""
		}
		morae = append(morae, entry)
	}
	return map[string]any{
		"text":              text,
		"voicebank_id":      "test-bank",
		"tone":              "C4",
		"mora_duration_ms":  140,
		"pause_duration_ms": 180,
		"intonation":        0,
		"apply_pitch":       false,
		"analysis_cache": map[string]any{
			"reading": reading,
			"morae":   morae,
		},
	}
}

func TestExportUstxDistinctVoicebanksGetDistinctTracks(t *testing.T) {
	engine := exportEngine(t)
	text := exportProject(t, engine, map[string]any{
		"format":         "utautts-project",
		"format_version": 5,
		"utterances": []any{
			map[string]any{
				"text":              "おはよう",
				"voicebank_id":      "bank-alpha",
				"tone":              "C4",
				"mora_duration_ms":  140,
				"pause_duration_ms": 180,
				"analysis_cache": map[string]any{
					"reading": "オハヨー",
					"morae":   []any{map[string]any{"position": 0, "mora": "お"}},
				},
			},
			map[string]any{
				"text":              "こんにちは",
				"voicebank_id":      "bank-beta",
				"tone":              "C4",
				"mora_duration_ms":  140,
				"pause_duration_ms": 180,
				"analysis_cache": map[string]any{
					"reading": "コンニチハ",
					"morae":   []any{map[string]any{"position": 0, "mora": "こ"}},
				},
			},
		},
	})

	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(text), &parsed); err != nil {
		t.Fatal(err)
	}
	tracks := parsed["tracks"].([]any)
	if len(tracks) != 2 {
		t.Fatalf("tracks = %d, want 2 for two voicebanks", len(tracks))
	}
	parts := parsed["voice_parts"].([]any)
	first := parts[0].(map[string]any)
	second := parts[1].(map[string]any)
	if first["track_no"] == second["track_no"] {
		t.Fatal("different voicebanks must land on different tracks")
	}
	if second["position"].(int) <= first["position"].(int) {
		t.Fatalf("cards on different tracks must remain sequential: %v / %v", first, second)
	}
	if tracks[0].(map[string]any)["singer"] != "bank-alpha" || tracks[1].(map[string]any)["singer"] != "bank-beta" {
		t.Fatalf("track singers = %v / %v", tracks[0], tracks[1])
	}
}

func TestExportUstxSkipsEmptyCardsButKeepsTheRest(t *testing.T) {
	engine := exportEngine(t)
	text := exportProject(t, engine, map[string]any{
		"format":         "utautts-project",
		"format_version": 5,
		"utterances": []any{
			map[string]any{
				"text": "", "voicebank_id": "bank", "tone": "C4",
				"mora_duration_ms": 140, "pause_duration_ms": 180,
				"analysis_cache": map[string]any{},
			},
			map[string]any{
				"text":              "こんにちは",
				"voicebank_id":      "bank",
				"tone":              "C4",
				"mora_duration_ms":  140,
				"pause_duration_ms": 180,
				"analysis_cache": map[string]any{
					"reading": "コンニチハ",
					"morae":   []any{map[string]any{"position": 0, "mora": "こ"}},
				},
			},
		},
	})

	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(text), &parsed); err != nil {
		t.Fatal(err)
	}
	parts := parsed["voice_parts"].([]any)
	if len(parts) != 1 {
		t.Fatalf("voice_parts = %d, want 1 (empty card skipped)", len(parts))
	}
	emptyProject, err := json.Marshal(map[string]any{
		"format":         "utautts-project",
		"format_version": 5,
		"utterances": []any{
			map[string]any{"text": "", "voicebank_id": "bank", "tone": "C4"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	request, err := json.Marshal(exportRequest{OutputPath: "unused.ustx", Project: emptyProject})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.exportUstx(request); err == nil {
		t.Fatal("expected an error when no utterance has notes")
	}
}

func TestExportUstxOutputPathWithBackslashes(t *testing.T) {
	engine := exportEngine(t)
	outputPath := filepath.Join(t.TempDir(), `win\out.ustx`)
	projectData, err := json.Marshal(map[string]any{
		"format":         "utautts-project",
		"format_version": 5,
		"utterances": []any{
			map[string]any{
				"text":              "こんにちは",
				"voicebank_id":      "bank",
				"tone":              "C4",
				"mora_duration_ms":  140,
				"pause_duration_ms": 180,
				"analysis_cache": map[string]any{
					"reading": "コンニチハ",
					"morae":   []any{map[string]any{"position": 0, "mora": "こ", "vowel": "o"}},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	exportProjectTo(t, engine, outputPath, projectData)
}
