package native

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// exportEngine constructs an Engine without any external assets (empty voice
// directory, no plugins, no Open JTalk helper). USTX export works purely from
// the project data, so the tests below stay independent of the local machine.
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

// exportRequest mirrors the native exportUstx request envelope.
type exportRequest struct {
	OutputPath string          `json:"output_path"`
	Project    json.RawMessage `json:"project"`
}

// exportProjectTo runs one export request against the engine and returns the
// exported USTX text. The request is always serialized with encoding/json so
// platform-specific output paths (Windows backslashes under t.TempDir())
// cannot produce invalid JSON escapes.
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
			// the vowel is carried over from the previous mora by the analyzer;
			// tests only need the extension-note behavior to work
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

// TestExportUstxMultiUtteranceSequentialParts covers several cards on one
// voicebank: every card becomes its own part, laid out sequentially on the
// track instead of stacked at tick 0.
func TestExportUstxMultiUtteranceSequentialParts(t *testing.T) {
	engine := exportEngine(t)
	text := exportProject(t, engine, map[string]any{
		"format":         "utautts-project",
		"format_version": 5,
		"utterances": []any{
			map[string]any{
				"text":              "おはよう",
				"voicebank_id":      "test-bank-a",
				"tone":              "C4",
				"mora_duration_ms":  140,
				"pause_duration_ms": 180,
				"analysis_cache": map[string]any{
					"reading": "オハヨー",
					"morae": []any{
						map[string]any{"position": 0, "mora": "お", "vowel": "o"},
						map[string]any{"position": 1, "mora": "は", "vowel": "a"},
						map[string]any{"position": 2, "mora": "よ", "vowel": "o"},
						map[string]any{"position": 3, "mora": "ー", "vowel": "o"},
					},
				},
			},
			map[string]any{
				"text":              "こんにちは",
				"voicebank_id":      "test-bank-a",
				"tone":              "C4",
				"mora_duration_ms":  140,
				"pause_duration_ms": 180,
				"analysis_cache": map[string]any{
					"reading": "コンニチハ",
					"morae": []any{
						map[string]any{"position": 0, "mora": "こ", "vowel": "o"},
						map[string]any{"position": 1, "mora": "ん", "vowel": "n"},
						map[string]any{"position": 2, "mora": "に", "vowel": "i"},
						map[string]any{"position": 3, "mora": "ち", "vowel": "i"},
						map[string]any{"position": 4, "mora": "は", "vowel": "a"},
					},
				},
			},
		},
	})

	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(text), &parsed); err != nil {
		t.Fatal(err)
	}
	parts := parsed["voice_parts"].([]any)
	if len(parts) != 2 {
		t.Fatalf("voice_parts = %d, want 2", len(parts))
	}
	first := parts[0].(map[string]any)
	second := parts[1].(map[string]any)
	if first["track_no"] != second["track_no"] {
		t.Fatalf("same-voicebank cards must share a track")
	}
	firstStart := first["position"].(int)
	secondStart := second["position"].(int)
	if firstStart != 0 {
		t.Errorf("first part position = %d, want 0", firstStart)
	}
	if secondStart <= firstStart {
		t.Errorf("second part position %d overlaps first part position %d", secondStart, firstStart)
	}

	text2 := parsed["tracks"].([]any)
	if len(text2) != 1 {
		t.Fatalf("tracks = %d, want 1 for a single voicebank", len(text2))
	}
}

// TestExportUstxDistinctVoicebanksGetDistinctTracks covers several banks in
// one project: each bank becomes its own USTX track.
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

// TestExportUstxSkipsEmptyCardsButKeepsTheRest covers projects that contain
// an empty/unanalyzed card next to analyzed ones.
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
}

// TestExportUstxRejectsEmptyProject makes sure exporting a project with no
// synthesizable content fails loudly instead of writing an empty file.
func TestExportUstxRejectsEmptyProject(t *testing.T) {
	engine := exportEngine(t)
	projectData, err := json.Marshal(map[string]any{
		"format":         "utautts-project",
		"format_version": 5,
		"utterances": []any{
			map[string]any{"text": "", "voicebank_id": "bank", "tone": "C4"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	request, err := json.Marshal(exportRequest{OutputPath: "unused.ustx", Project: projectData})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.exportUstx(request); err == nil {
		t.Fatal("expected an error when no utterance has notes")
	}
}

// TestExportUstxOutputPathWithBackslashes guards the request serialization on
// Windows, where t.TempDir() returns backslash paths (C:\Users\...). Raw
// string concatenation would embed them as invalid JSON escapes (\U, \A, ...);
// serializing the whole request with encoding/json keeps them intact.
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
