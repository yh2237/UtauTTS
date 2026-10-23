package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLITimeoutValidationAndSuccess(t *testing.T) {
	base := synthesisArgs(t)
	for _, test := range []struct {
		duration, want string
		code           int
	}{
		{"-1s", "invalid_arguments", 1},
		{"1ns", "deadline_exceeded", 1},
		{"30s", "", 0},
	} {
		t.Run(test.duration, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			path := filepath.Join(t.TempDir(), "speech.wav")
			args := append(base, "--json", "--reading", "あ", "--out", path, "--timeout", test.duration)
			code := executeCLI(args, strings.NewReader(""), &stdout, &stderr)
			response := decodeResponse(t, stdout.Bytes())
			if code != test.code {
				t.Fatalf("exit=%d, response=%#v, stderr=%q", code, response, stderr.String())
			}
			if test.code == 0 {
				if response["ok"] != true {
					t.Fatalf("success response=%#v", response)
				}
			} else {
				if response["ok"] != false || response["error"].(map[string]any)["code"] != test.want {
					t.Fatalf("timeout response=%#v", response)
				}
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("failed synthesis wrote WAV: %v", err)
				}
			}
		})
	}
	var stdout, stderr bytes.Buffer
	args := append(base[:4:4], "--json", "--request", "-", "--timeout", "30s", "--out", filepath.Join(t.TempDir(), "request.wav"))
	request := `{"reading":"あ","renderer":"cli-test-waveform","method":"waveform-synthesis","language":"ja","apply_pitch":false}`
	if code := executeCLI(args, strings.NewReader(request), &stdout, &stderr); code != 0 {
		t.Fatalf("JSON request timeout: exit=%d, stdout=%q, stderr=%q", code, stdout.String(), stderr.String())
	}
}
