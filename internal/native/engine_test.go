package native

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/yh2237/UtauTTS/internal/audio"
)

func newTestEngine(t *testing.T, config Config) *Engine {
	t.Helper()
	engine, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := engine.Call("shutdown", nil); err != nil {
			t.Error(err)
		}
	})
	return engine
}

func waveformTestRendererDirectory(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	manifest := []byte(`{"manifest_version":2,"kind":"synthesis-engine","id":"test-concat","display_name":"Test waveform","contract":"unit-renderer","provider":"test-concat","provider_version":"1"}`)
	if err := os.WriteFile(filepath.Join(directory, "renderer.json"), manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	return directory
}

// openJTalkRuntimeはリポジトリのruntimeにあるOpen JTalkのライブラリと辞書を返す。
func openJTalkRuntime(t *testing.T) (string, string) {
	t.Helper()
	name := map[string]string{"windows": "utautts-openjtalk-engine.dll", "darwin": "utautts-openjtalk-engine.dylib"}[runtime.GOOS]
	if name == "" {
		name = "utautts-openjtalk-engine.so"
	}
	library := filepath.Join("..", "..", "runtime", name)
	dictionary := filepath.Join("..", "..", "runtime", "open_jtalk_dic_utf_8-1.11")
	for _, path := range []string{library, dictionary} {
		if _, err := os.Stat(path); err != nil {
			t.Skipf("Open JTalk runtime is missing: %v", err)
		}
	}
	return library, dictionary
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
	engine := newTestEngine(t, Config{VoiceDir: root})
	voicebanks, err := engine.Call("voicebanks", nil)
	if err != nil || !strings.Contains(string(voicebanks), `"has_vc":true`) || !strings.Contains(string(voicebanks), `"VC":1`) {
		t.Fatalf("voicebank capabilities=%s err=%v", voicebanks, err)
	}
	for _, method := range []string{"health", "settings", "models"} {
		if _, err := engine.Call(method, nil); err != nil {
			t.Fatalf("%s: %v", method, err)
		}
	}
	analysis, err := engine.Call("analyze", []byte(`{"language":"ja","text":"あ"}`))
	if err != nil {
		t.Fatal(err)
	}
	var reading struct {
		Reading string `json:"reading"`
	}
	if err := json.Unmarshal(analysis, &reading); err != nil || reading.Reading != "ア" {
		t.Fatalf("analysis=%s err=%v", analysis, err)
	}
	preview, err := engine.Call("predictProsody", []byte(`{"kana":"\u3042","method":"waveform-synthesis","language":"ja","mora_duration_ms":100,"apply_pitch":false}`))
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
	dictionaryAnalysis, err := engine.Call("analyze", []byte(`{"language":"ja","text":"v8","dictionary":[{"surface":"v8","reading":"ぶいはち"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	var dictionaryResult struct {
		Reading string `json:"reading"`
	}
	if err := json.Unmarshal(dictionaryAnalysis, &dictionaryResult); err != nil {
		t.Fatal(err)
	}
	if dictionaryResult.Reading != "ブイハチ" {
		t.Fatalf("dictionary reading was reinterpreted: %s", dictionaryAnalysis)
	}
	ignoredAnalysis, err := engine.Call("analyze", []byte(`{"language":"ja","text":"🙂"}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(ignoredAnalysis) != `{"morae":[],"reading":""}` {
		t.Fatalf("invalid character was not ignored: %s", ignoredAnalysis)
	}
	output := filepath.Join(root, "preview.wav")
	request, _ := json.Marshal(map[string]any{"kana": "あ", "method": "waveform-synthesis", "language": "ja", "voicebank_id": "bank", "mora_duration_ms": 100, "output_path": output})
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
	request, _ = json.Marshal(map[string]any{"kana": "い", "method": "waveform-synthesis", "language": "ja", "voicebank_id": "bank", "mora_duration_ms": 100, "output_path": filepath.Join(root, "reloaded.wav")})
	if _, err := engine.Call("synthesize", request); err != nil {
		t.Fatalf("synthesis after voicebank reload: %v", err)
	}
}

func TestEngineListsAllVoicebankTypes(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(bankDir, "oto.ini"), []byte("a.wav=縺・0,0,0,0,0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bankDir, "character.yaml"), []byte("subbanks:\n- color: \"\"\n  suffix: \" normal\"\n- color: \"power\"\n  suffix: \" power\"\n- color: \"edge\"\n  suffix: \" edge\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	engine := newTestEngine(t, Config{VoiceDir: root})
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
	var oto strings.Builder
	for _, alias := range []string{"え", "い", "ち", "ー", "る", "お"} {
		oto.WriteString("a.wav=" + alias + ",0,0,0,0,0\n")
	}
	if err := os.WriteFile(filepath.Join(bankDir, "oto.ini"), []byte(oto.String()), 0644); err != nil {
		t.Fatal(err)
	}
	library, dictionary := openJTalkRuntime(t)
	engine := newTestEngine(t, Config{VoiceDir: root, OpenJTalkPath: library, OpenJTalkDictionary: dictionary})

	analysisJSON, err := engine.Call("analyze", []byte(`{"language":"ja","text":"hello"}`))
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
	if analysis.Reading != "エイチイーエルエルオー" || len(analysis.Morae) != 11 {
		t.Fatalf("analysis=%s", analysisJSON)
	}

	output := filepath.Join(root, "english.wav")
	request, _ := json.Marshal(map[string]any{
		"text": "hello", "method": "waveform-synthesis", "language": "ja", "voicebank_id": "bank", "mora_duration_ms": 100, "output_path": output,
	})
	if _, err := engine.Call("synthesize", request); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(output); err != nil || info.Size() < 44 {
		t.Fatalf("output info=%v err=%v", info, err)
	}
}

func TestEngineAnalyzesChinese(t *testing.T) {
	engine := newTestEngine(t, Config{VoiceDir: t.TempDir()})
	result, err := engine.Call("analyze", []byte(`{"text":"你好","language":"zh","phonemizer":"zh-cvvc"}`))
	if err != nil {
		t.Fatal(err)
	}
	var analysis struct {
		Reading string `json:"reading"`
		Morae   []struct {
			Mora string `json:"mora"`
		} `json:"morae"`
	}
	if err := json.Unmarshal(result, &analysis); err != nil {
		t.Fatal(err)
	}
	if analysis.Reading != "ni3 hao3" || len(analysis.Morae) != 2 || analysis.Morae[0].Mora != "ni" {
		t.Fatalf("analysis=%s", result)
	}
}

func TestShutdownStopsFurtherCalls(t *testing.T) {
	engine := newTestEngine(t, Config{VoiceDir: t.TempDir()})
	data, err := engine.Call("settings", nil)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Settings []struct {
			ID string `json:"id"`
		} `json:"settings"`
	}
	if err := json.Unmarshal(data, &result); err != nil || len(result.Settings) == 0 {
		t.Fatalf("settings = %s, err = %v", data, err)
	}
	if _, err := engine.Call("shutdown", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Call("predictProsody", []byte(`{"kana":"あ"}`)); err == nil {
		t.Fatal("shutdown still allows prosody prediction")
	}
}

func TestNewAllowsMissingVoiceDirectory(t *testing.T) {
	engine := newTestEngine(t, Config{VoiceDir: filepath.Join(t.TempDir(), "missing")})
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
	engine := newTestEngine(t, Config{VoiceDir: filepath.Join(root, "missing")})
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

func TestNativeRequestsUseSharedFields(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(bankDir, "oto.ini"), []byte("a.wav=あ,0,0,0,0,0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	engine := newTestEngine(t, Config{VoiceDir: root})
	output := filepath.Join(root, "preview.wav")

	shared, _ := json.Marshal(map[string]any{
		"kana": "あ", "method": "waveform-synthesis", "language": "ja", "voicebank_id": "bank", "mora_duration_ms": 100, "output_path": output,
		"mora_durations_ms": []float64{150}, "alias_policy": "auto", "release_ms": 5, "release_set": true,
	})
	synthesis, err := engine.Call("synthesize", shared)
	if err != nil {
		t.Fatalf("shared synthesis fields were rejected: %v", err)
	}
	var synthesisResult struct {
		Lab string `json:"lab"`
	}
	if err := json.Unmarshal(synthesis, &synthesisResult); err != nil || !strings.Contains(synthesisResult.Lab, " a\n") {
		t.Fatalf("synthesis=%s err=%v", synthesis, err)
	}

	unknown, _ := json.Marshal(map[string]any{"kana": "あ", "method": "waveform-synthesis", "language": "ja", "voicebank_id": "bank", "output_path": output, "unknown_option": 1})
	if _, err := engine.Call("synthesize", unknown); err != nil {
		t.Fatalf("native synthesis is lenient for extra fields: %v", err)
	}

	preview, err := engine.Call("predictProsody", []byte(`{"request_id":"abc","kana":"あ","method":"waveform-synthesis","language":"ja","mora_duration_ms":100,"apply_pitch":false}`))
	if err != nil {
		t.Fatal(err)
	}
	var previewResult struct {
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(preview, &previewResult); err != nil || previewResult.RequestID != "abc" {
		t.Fatalf("prosody preview=%s err=%v", preview, err)
	}
	if _, err := engine.Call("predictProsody", []byte(`{"kana":"あ","method":"waveform-synthesis","language":"ja","unknown_option":1}`)); err != nil {
		t.Fatalf("native prosody preview is lenient for extra fields: %v", err)
	}
}
