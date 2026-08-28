package native

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"utautts/internal/audio"
)

const openJTalkHelperEnvironment = "UTAUTTS_TEST_OPENJTALK_HELPER"

func TestMain(m *testing.M) {
	if os.Getenv(openJTalkHelperEnvironment) == "1" {
		for _, argument := range os.Args {
			if argument == "--serve" {
				scanner := bufio.NewScanner(os.Stdin)
				for scanner.Scan() {
					_, _ = fmt.Fprintln(os.Stdout, `{"version":1,"reading":"ハロー","morae":["は","ろ","ー"],"features":[{},{},{}]}`)
				}
				os.Exit(0)
			}
		}
		_, _ = io.Copy(io.Discard, os.Stdin)
		_, _ = fmt.Fprint(os.Stdout, `{"version":1,"reading":"ハロー","morae":["は","ろ","ー"],"features":[{},{},{}]}`)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestEngineListsAnalyzesAndSynthesizes(t *testing.T) {
	root := t.TempDir()
	bankDir := filepath.Join(root, "bank")
	if err := os.Mkdir(bankDir, 0755); err != nil {
		t.Fatal(err)
	}
	samples := make([]int16, 400)
	for index := range samples {
		samples[index] = 4000
	}
	if err := audio.WriteWav(filepath.Join(bankDir, "a.wav"), &audio.PCM{SampleRate: 1000, Channels: 1, Data: samples}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bankDir, "oto.ini"), []byte("a.wav=あ,0,0,0,0,0\na.wav=a k,0,0,0,0,0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	engine, err := New(Config{VoiceDir: root, Renderer: "waveform"})
	if err != nil {
		t.Fatal(err)
	}
	voicebanks, err := engine.Call("voicebanks", nil)
	if err != nil || !strings.Contains(string(voicebanks), `"has_vc":true`) || !strings.Contains(string(voicebanks), `"VC":1`) {
		t.Fatalf("voicebank capabilities=%s err=%v", voicebanks, err)
	}
	for _, method := range []string{"health", "voicebanks", "renderers", "models"} {
		if _, err := engine.Call(method, nil); err != nil {
			t.Fatalf("%s: %v", method, err)
		}
	}
	analysis, err := engine.Call("analyze", []byte(`{"text":"あ"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(analysis) {
		t.Fatalf("analysis=%s", analysis)
	}
	preview, err := engine.Call("predictProsody", []byte(`{"kana":"\u3042","mora_duration_ms":100,"apply_pitch":false}`))
	if err != nil {
		t.Fatal(err)
	}
	var previewResult struct {
		MoraDurationsMS []float64 `json:"mora_durations_ms"`
	}
	if err := json.Unmarshal(preview, &previewResult); err != nil {
		t.Fatalf("prosody preview=%s: %v", preview, err)
	}
	if len(previewResult.MoraDurationsMS) != 1 || previewResult.MoraDurationsMS[0] != 100 {
		t.Fatalf("prosody preview=%s", preview)
	}
	dictionaryAnalysis, err := engine.Call("analyze", []byte(`{"text":"UtauTTS","dictionary":[{"surface":"UtauTTS","reading":"あ"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	var dictionaryResult struct {
		Reading string `json:"reading"`
	}
	if err := json.Unmarshal(dictionaryAnalysis, &dictionaryResult); err != nil {
		t.Fatal(err)
	}
	if dictionaryResult.Reading != "ア" {
		t.Fatalf("dictionary analysis=%s", dictionaryAnalysis)
	}
	output := filepath.Join(root, "preview.wav")
	request, _ := json.Marshal(map[string]any{"kana": "あ", "voicebank_id": "bank", "mora_duration_ms": 100, "output_path": output})
	synthesis, err := engine.Call("synthesize", request)
	if err != nil {
		t.Fatal(err)
	}
	var synthesisResult struct {
		Lab string `json:"lab"`
	}
	if err := json.Unmarshal(synthesis, &synthesisResult); err != nil || !strings.Contains(synthesisResult.Lab, " a\n") {
		t.Fatalf("synthesis label=%q err=%v", synthesisResult.Lab, err)
	}
	if info, err := os.Stat(output); err != nil || info.Size() < 44 {
		t.Fatalf("output info=%v err=%v", info, err)
	}
	sidecarRequest, _ := json.Marshal(map[string]any{
		"wav_path": output, "text": "あ", "lab": synthesisResult.Lab,
		"encoding": "utf-8", "write_text": true, "write_lab": true,
	})
	if _, err := engine.Call("writeSidecars", sidecarRequest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(strings.TrimSuffix(output, ".wav") + ".txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(strings.TrimSuffix(output, ".wav") + ".lab"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bankDir, "oto.ini"), []byte("a.wav=い,0,0,0,0,0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Call("reloadVoicebanks", nil); err != nil {
		t.Fatal(err)
	}
	request, _ = json.Marshal(map[string]any{"kana": "い", "voicebank_id": "bank", "mora_duration_ms": 100, "output_path": filepath.Join(root, "reloaded.wav")})
	if _, err := engine.Call("synthesize", request); err != nil {
		t.Fatalf("synthesis after voicebank reload: %v", err)
	}
}

func TestEngineListsAllVoicebankTypes(t *testing.T) {
	root := t.TempDir()
	bankDir := filepath.Join(root, "teto")
	if err := os.Mkdir(bankDir, 0755); err != nil {
		t.Fatal(err)
	}
	samples := make([]int16, 400)
	for index := range samples {
		samples[index] = 4000
	}
	if err := audio.WriteWav(filepath.Join(bankDir, "a.wav"), &audio.PCM{SampleRate: 1000, Channels: 1, Data: samples}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bankDir, "oto.ini"), []byte("a.wav=縺・0,0,0,0,0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bankDir, "character.yaml"), []byte("subbanks:\n- color: \"\"\n  suffix: \" normal\"\n- color: \"power\"\n  suffix: \" power\"\n- color: \"edge\"\n  suffix: \" edge\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	engine, err := New(Config{VoiceDir: root})
	if err != nil {
		t.Fatal(err)
	}
	resultJSON, err := engine.Call("voicebanks", nil)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Voicebanks []struct {
			Types []struct {
				ID    string `json:"id"`
				Color string `json:"color"`
			} `json:"types"`
		} `json:"voicebanks"`
	}
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatalf("voicebanks=%s: %v", resultJSON, err)
	}
	if len(result.Voicebanks) != 1 || len(result.Voicebanks[0].Types) != 3 {
		t.Fatalf("voicebank types=%s", resultJSON)
	}
	if result.Voicebanks[0].Types[2].ID != "subbank-2" || result.Voicebanks[0].Types[2].Color != "edge" {
		t.Fatalf("voicebank types=%s", resultJSON)
	}
}

func TestEngineFallsBackToOpenJTalkForEnglish(t *testing.T) {
	root := t.TempDir()
	bankDir := filepath.Join(root, "bank")
	if err := os.Mkdir(bankDir, 0755); err != nil {
		t.Fatal(err)
	}
	samples := make([]int16, 400)
	for index := range samples {
		samples[index] = 4000
	}
	if err := audio.WriteWav(filepath.Join(bankDir, "a.wav"), &audio.PCM{SampleRate: 1000, Channels: 1, Data: samples}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bankDir, "oto.ini"), []byte("a.wav=は,0,0,0,0,0\na.wav=ろ,0,0,0,0,0\na.wav=ー,0,0,0,0,0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(openJTalkHelperEnvironment, "1")
	engine, err := New(Config{VoiceDir: root, Renderer: "waveform", OpenJTalkPath: helper, OpenJTalkDictionary: root})
	if err != nil {
		t.Fatal(err)
	}

	analysisJSON, err := engine.Call("analyze", []byte(`{"text":"hello"}`))
	if err != nil {
		t.Fatal(err)
	}
	var analysis struct {
		Reading string `json:"reading"`
		Morae   []any  `json:"morae"`
	}
	if err := json.Unmarshal(analysisJSON, &analysis); err != nil {
		t.Fatal(err)
	}
	if analysis.Reading != "ハロー" || len(analysis.Morae) != 3 {
		t.Fatalf("analysis=%s", analysisJSON)
	}

	output := filepath.Join(root, "english.wav")
	request, _ := json.Marshal(map[string]any{
		"text": "hello", "voicebank_id": "bank", "mora_duration_ms": 100, "output_path": output,
	})
	if _, err := engine.Call("synthesize", request); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(output); err != nil || info.Size() < 44 {
		t.Fatalf("output info=%v err=%v", info, err)
	}
}

func TestNewReportsInvalidPlugin(t *testing.T) {
	pluginDirectory := t.TempDir()
	if err := os.WriteFile(filepath.Join(pluginDirectory, "plugin.json"), []byte(`{"kind":"renderer"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := New(Config{VoiceDir: t.TempDir(), RendererDirectories: []string{pluginDirectory}}); err == nil {
		t.Fatal("invalid renderer plugin was silently ignored")
	}
}

func TestNewAllowsMissingVoiceDirectory(t *testing.T) {
	engine, err := New(Config{VoiceDir: filepath.Join(t.TempDir(), "missing")})
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.Call("voicebanks", nil)
	if err != nil || string(result) != `{"voicebanks":[]}` {
		t.Fatalf("voicebanks=%s err=%v", result, err)
	}
}

func TestEngineWritesDragExo(t *testing.T) {
	root := t.TempDir()
	paths := make([]string, 2)
	for index, duration := range []int{1000, 2000} {
		paths[index] = filepath.Join(root, fmt.Sprintf("00%d_あ.wav", index+1))
		data := make([]int16, duration)
		for frame := range data {
			data[frame] = 4000
		}
		if err := audio.WriteWav(paths[index], &audio.PCM{SampleRate: 1000, Channels: 1, Data: data}); err != nil {
			t.Fatal(err)
		}
	}
	engine, err := New(Config{VoiceDir: filepath.Join(root, "missing")})
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "utautts.exo")
	request, _ := json.Marshal(map[string]any{"output_path": output, "files": paths, "frame_rate": 60})
	if _, err := engine.Call("writeExo", request); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(content) < 16 || !strings.Contains(string(content), "audio_rate") {
		t.Fatalf("unexpected exo output: %q", content)
	}
}
