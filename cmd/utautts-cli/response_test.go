package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func decodeResponse(t *testing.T, data []byte) map[string]any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(data))
	var response map[string]any
	if err := decoder.Decode(&response); err != nil {
		t.Fatalf("response is not JSON: %v, %q", err, data)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		t.Fatalf("extra output after JSON: %v, %q", err, data)
	}
	return response
}

func TestCLIJSONSynthesisResult(t *testing.T) {
	args := synthesisArgs(t)
	path := filepath.Join(t.TempDir(), "speech.wav")
	var stdout, stderr bytes.Buffer
	code := executeCLI(append(args, "--json", "--reading", "あ", "--out", path, "--write-lab"), strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d, stderr=%q", code, stderr.String())
	}
	response := decodeResponse(t, stdout.Bytes())
	result, ok := response["result"].(map[string]any)
	if !ok || response["ok"] != true || response["error"] != nil {
		t.Fatalf("success response = %#v", response)
	}
	if result["path"] != path || result["reading"] != "あ" || result["renderer_id"] != "cli-test-waveform" || result["unit_count"] != float64(1) {
		t.Fatalf("result metadata = %#v", result)
	}
	if result["sample_rate"] != float64(16000) || result["channels"] != float64(1) || result["duration_ms"].(float64) <= 0 {
		t.Fatalf("audio metadata = %#v", result)
	}
	if diagnostics, ok := result["diagnostics"].([]any); !ok || len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", result["diagnostics"])
	}
	label, err := os.ReadFile(strings.TrimSuffix(path, ".wav") + ".lab")
	if err != nil || result["phoneme_labels"] != string(label) {
		t.Fatalf("phoneme labels differ: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestCLIJSONErrors(t *testing.T) {
	base := synthesisArgs(t)
	for _, test := range []struct {
		name, code, stdin string
		args              []string
	}{
		{"required flags", "invalid_arguments", "", []string{"--json"}},
		{"stdout conflict", "invalid_arguments", "", append(base, "--json", "--reading", "あ", "--out", "-")},
		{"UTF-8 input", "input_error", "\xff", append(base, "--json", "--text-file", "-", "--out", filepath.Join(t.TempDir(), "invalid.wav"))},
		{"unknown renderer", "unavailable", "", append(base, "--json", "--renderer", "does-not-exist", "--reading", "あ", "--out", filepath.Join(t.TempDir(), "unavailable.wav"))},
		{"synthesis failure", "synthesis_error", "", append(base, "--json", "--reading", "unknown", "--out", filepath.Join(t.TempDir(), "invalid-reading.wav"))},
		{"file output failure", "output_error", "", append(base, "--json", "--reading", "あ", "--out", filepath.Join(t.TempDir(), "missing", "speech.wav"))},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := executeCLI(test.args, strings.NewReader(test.stdin), &stdout, &stderr); code != 1 {
				t.Fatalf("exit=%d, stdout=%q, stderr=%q", code, stdout.String(), stderr.String())
			}
			response := decodeResponse(t, stdout.Bytes())
			detail, ok := response["error"].(map[string]any)
			if response["ok"] != false || response["result"] != nil || !ok || detail["code"] != test.code || detail["message"] == "" {
				t.Fatalf("error response = %#v", response)
			}
		})
	}
}

func TestCLIJSONReportsMissingPhones(t *testing.T) {
	args := synthesisArgs(t)
	if err := os.WriteFile(filepath.Join(args[1], "oto.ini"), []byte("a.wav=tA,0,40,0,60,30\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	path := filepath.Join(t.TempDir(), "speech.wav")
	code := executeCLI(append(args, "--json", "--language", "en", "--phonemizer", "en-delta", "--reading", "S T AA1", "--out", path), strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d, stderr=%q", code, stderr.String())
	}
	response := decodeResponse(t, stdout.Bytes())
	result := response["result"].(map[string]any)
	diagnostics := result["diagnostics"].([]any)
	for _, item := range diagnostics {
		diagnostic := item.(map[string]any)
		if diagnostic["code"] == "missing-phones" && diagnostic["severity"] == "warning" && diagnostic["message"] != "" {
			return
		}
	}
	t.Fatalf("missing phone was not reported: %#v", diagnostics)
}

func TestCLIJSONVersionAndParseErrors(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := executeCLI([]string{"--json", "--version"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("version exit = %d", code)
	}
	response := decodeResponse(t, stdout.Bytes())
	version, ok := response["version"].(map[string]any)
	if response["ok"] != true || !ok || version["name"] == "" || version["version"] == "" {
		t.Fatalf("version response = %#v", response)
	}
	stdout.Reset()
	stderr.Reset()
	if code := executeCLI([]string{"--json", "--unknown-flag"}, strings.NewReader(""), &stdout, &stderr); code != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("flag error: exit=%d, stdout=%q, stderr=%q", code, stdout.String(), stderr.String())
	}
	if code := executeCLI([]string{"--json", "--version"}, strings.NewReader(""), failingWriter{io.ErrClosedPipe}, io.Discard); code != 1 {
		t.Fatalf("JSON writer failure exit=%d", code)
	}
}

func TestCLIClassicToolSelectionsAreNotIgnored(t *testing.T) {
	base := synthesisArgs(t)
	root := base[1]
	t.Chdir(root)
	if err := os.WriteFile(filepath.Join(root, "renderer.json"), []byte(`{
		"manifest_version":2,"kind":"synthesis-engine","id":"cli-test-classic",
		"display_name":"CLI test Classic","contract":"unit-renderer",
		"provider":"utau-external-resampler","provider_version":"1"
	}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "Resamplers"), 0700); err != nil {
		t.Fatal(err)
	}
	// 選択したツールの不足を起動前に検出する。
	tool := "fixture"
	if runtime.GOOS == "windows" {
		tool += ".exe"
	}
	if err := os.WriteFile(filepath.Join(root, "Resamplers", tool), []byte("unused"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		args []string
	}{
		{"missing-resampler", []string{"--resampler", "missing-resampler"}},
		{"missing-wavtool", []string{"--resampler", tool, "--wavtool", "missing-wavtool"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			args := append(base, "--json", "--renderer", "cli-test-classic", "--reading", "あ", "--out", filepath.Join(root, "speech.wav"))
			var stdout, stderr bytes.Buffer
			code := executeCLI(append(args, test.args...), strings.NewReader(""), &stdout, &stderr)
			response := decodeResponse(t, stdout.Bytes())
			detail, ok := response["error"].(map[string]any)
			if code != 1 || !ok || detail["code"] != "unavailable" || !strings.Contains(detail["message"].(string), test.name) {
				t.Fatalf("selected tool was ignored: exit=%d, response=%#v, stderr=%q", code, response, stderr.String())
			}
		})
	}
}
