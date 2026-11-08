package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
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
	if result["path"] != path || result["reading"] != "あ" || result["method"] != "waveform-synthesis" || result["unit_count"] != float64(1) {
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
		exit              int
		args              []string
	}{
		{"required flags", "invalid_arguments", "", exitUsage, []string{"--json"}},
		{"stdout conflict", "invalid_arguments", "", exitUsage, append(base, "--json", "--reading", "あ", "--out", "-")},
		{"UTF-8 input", "input_error", "\xff", exitInput, append(base, "--json", "--text-file", "-", "--out", filepath.Join(t.TempDir(), "invalid.wav"))},
		{"synthesis failure", "synthesis_error", "", exitSynthesis, append(base, "--json", "--reading", "unknown", "--out", filepath.Join(t.TempDir(), "invalid-reading.wav"))},
		{"file output failure", "output_error", "", exitOutput, append(base, "--json", "--reading", "あ", "--out", filepath.Join(t.TempDir(), "missing", "speech.wav"))},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := executeCLI(test.args, strings.NewReader(test.stdin), &stdout, &stderr); code != test.exit {
				t.Fatalf("exit=%d, stdout=%q, stderr=%q", code, stdout.String(), stderr.String())
			}
			response := decodeResponse(t, stdout.Bytes())
			detail, ok := response["error"].(map[string]any)
			if response["ok"] != false || response["result"] != nil || !ok || detail["code"] != test.code || detail["message"] == "" {
				t.Fatalf("error response = %#v", response)
			}
			if hint, ok := detail["hint"].(string); !ok || hint == "" {
				t.Fatalf("error has no hint: %#v", detail)
			}
			if stderr.Len() != 0 {
				t.Fatalf("--json wrote to stderr: %q", stderr.String())
			}
		})
	}
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
	if code := executeCLI([]string{"--json", "--unknown-flag"}, strings.NewReader(""), &stdout, &stderr); code != exitUsage {
		t.Fatalf("flag error exit=%d, stdout=%q, stderr=%q", code, stdout.String(), stderr.String())
	}
	response = decodeResponse(t, stdout.Bytes())
	detail, ok := response["error"].(map[string]any)
	if response["ok"] != false || !ok || detail["code"] != "flag_parse_error" {
		t.Fatalf("flag error response = %#v", response)
	}
	if hint, ok := detail["hint"].(string); !ok || hint == "" {
		t.Fatalf("flag error has no hint: %#v", detail)
	}
	if stderr.Len() != 0 {
		t.Fatalf("--json wrote to stderr: %q", stderr.String())
	}
	if code := executeCLI([]string{"--json", "--version"}, strings.NewReader(""), failingWriter{io.ErrClosedPipe}, io.Discard); code != exitOutput {
		t.Fatalf("JSON writer failure exit=%d", code)
	}
}

func TestCLITextErrorsCarryHints(t *testing.T) {
	base := synthesisArgs(t)
	for _, test := range []struct {
		name, message, hint string
		args                []string
	}{
		{"missing voicebank", "--voicebank is required", "--voicebank <string>", []string{"--out", "out.wav"}},
		{"missing out", "--out is required", "--out <string>", []string{"--voicebank", t.TempDir()}},
		{"missing text", "input text is required", "--text <string>", append(base[:4:4], "--out", "out.wav")},
		{"out of range", "--mora-ms must be at most 1000", "--mora-ms <number>", append(base, "--reading", "あ", "--mora-ms", "5000", "--out", "out.wav")},
		{"too long", "text is limited to 500 characters", "split the input", append(base, "--text", strings.Repeat("あ", 501), "--out", "out.wav")},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := executeCLI(test.args, strings.NewReader(""), &stdout, &stderr)
			if code != exitUsage || stdout.Len() != 0 {
				t.Fatalf("exit=%d, stdout=%q, stderr=%q", code, stdout.String(), stderr.String())
			}
			if !strings.Contains(stderr.String(), test.message) || !strings.Contains(stderr.String(), "hint: "+test.hint) {
				t.Fatalf("stderr = %q", stderr.String())
			}
		})
	}
}
