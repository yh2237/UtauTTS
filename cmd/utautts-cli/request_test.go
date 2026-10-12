package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIJSONRequestMatchesFlags(t *testing.T) {
	base := synthesisArgs(t)
	root := t.TempDir()
	baselinePath := filepath.Join(root, "baseline.wav")
	var stdout, stderr bytes.Buffer
	if code := executeCLI(append(base, "--reading", "あ", "--mora-ms", "160", "--out", baselinePath, "--write-lab"), strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("baseline exit=%d, stderr=%q", code, stderr.String())
	}
	baseline, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatal(err)
	}
	request := `{"kana":"あ","renderer":"cli-test-waveform","model_id":"none","apply_pitch":false,"mora_duration_ms":80,"renderer_settings":{"mora_duration_ms":160},"word_boundary_envelope":true}`
	path := filepath.Join(root, "request.json")
	if err := os.WriteFile(path, []byte(request), 0600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, path, input string
	}{
		{"file", path, ""},
		{"stdin", "-", "\ufeff" + request + "\r\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			stdout.Reset()
			stderr.Reset()
			args := append(base[:4:4], "--request", test.path, "--out", "-")
			if code := executeCLI(args, strings.NewReader(test.input), &stdout, &stderr); code != 0 || !bytes.Equal(stdout.Bytes(), baseline) {
				t.Fatalf("JSON request output differs: exit=%d, stdout=%d bytes, stderr=%q", code, stdout.Len(), stderr.String())
			}
		})
	}
}

func TestCLIJSONRequestRejectsConflictsAndInvalidData(t *testing.T) {
	base := synthesisArgs(t)
	for _, test := range []struct {
		name, input, want string
		flags             []string
	}{
		{"text conflict", `{}`, "invalid_arguments", []string{"--text", "あ"}},
		{"stdin conflict", `{}`, "invalid_arguments", []string{"--reading-file", "-"}},
		{"evaluation conflict", `{}`, "invalid_arguments", []string{"--prosody-pitch-only"}},
		{"null", `null`, "input_error", nil},
		{"multiple objects", `{} {}`, "input_error", nil},
		{"unknown field", `{"reading":"あ","voicebank_path":"ignored"}`, "input_error", nil},
		{"voicebank ID", `{"reading":"あ","voicebank_id":"not-supported"}`, "invalid_arguments", nil},
		{"empty utterance", `{}`, "invalid_arguments", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			args := append(base[:4:4], "--json", "--request", "-", "--out", filepath.Join(t.TempDir(), "speech.wav"))
			var stdout, stderr bytes.Buffer
			code := executeCLI(append(args, test.flags...), strings.NewReader(test.input), &stdout, &stderr)
			response := decodeResponse(t, stdout.Bytes())
			detail, ok := response["error"].(map[string]any)
			if code != 1 || response["ok"] != false || !ok || detail["code"] != test.want {
				t.Fatalf("rejected request: exit=%d, response=%#v, stderr=%q", code, response, stderr.String())
			}
		})
	}
}
