package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yh2237/UtauTTS/internal/provider"
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
	request := `{"reading":"あ","renderer":"cli-test-waveform","apply_pitch":false}`
	if code := executeCLI(args, strings.NewReader(request), &stdout, &stderr); code != 0 {
		t.Fatalf("JSON request timeout: exit=%d, stdout=%q, stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestCLITimeoutProvider(t *testing.T) {
	marker := os.Getenv("UTAUTTS_CLI_TIMEOUT_PROVIDER")
	if marker == "" {
		return
	}
	encoder := json.NewEncoder(os.Stdout)
	if err := encoder.Encode(provider.Hello{
		Type: provider.MessageHello, Protocol: provider.ProtocolName, ProtocolVersion: provider.ProtocolVersion,
		Provider: "cli-timeout-provider", ProviderVersion: "1", Session: true,
		Capabilities: []string{provider.CapabilityUnitRendererJobV2},
		Contracts:    []provider.ContractSupport{{Name: "unit-renderer", Version: 1}},
	}); err != nil {
		os.Exit(1)
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var message struct {
			Type      string `json:"type"`
			RequestID string `json:"request_id"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			os.Exit(1)
		}
		switch message.Type {
		case provider.MessageRender:
			if err := os.WriteFile(marker, []byte("render started"), 0600); err != nil {
				os.Exit(1)
			}
		case provider.MessageCancel:
			_ = encoder.Encode(provider.ErrorMessage{Type: provider.MessageError, RequestID: message.RequestID, Code: "canceled", Message: "canceled"})
		case provider.MessageShutdown:
			if err := os.WriteFile(marker+".closed", []byte("shutdown received"), 0600); err != nil {
				os.Exit(1)
			}
			os.Exit(0)
		}
	}
	os.Exit(0)
}

func TestCLITimeoutCancelsActiveProviderAndClosesRuntime(t *testing.T) {
	base := synthesisArgs(t)
	root := base[1]
	marker := filepath.Join(root, "started")
	t.Setenv("UTAUTTS_CLI_TIMEOUT_PROVIDER", marker)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := json.Marshal(map[string]any{
		"manifest_version": 2, "kind": "synthesis-engine", "id": "cli-timeout",
		"display_name": "CLI timeout fixture", "contract": "unit-renderer", "contract_version": 1,
		"provider": "cli-timeout-provider", "provider_version": "1",
		"protocol": provider.ProtocolName, "protocol_version": provider.ProtocolVersion,
		"provider_args": []string{"-test.run=^TestCLITimeoutProvider$"},
		"resources":     map[string]any{"provider_executable": map[string]any{"path": executable, "required": true, "executable": true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "renderer.json"), manifest, 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	path := filepath.Join(root, "must-not-exist.wav")
	args := append(base, "--renderer", "cli-timeout", "--json", "--reading", "あ", "--out", path, "--timeout", "2s")
	code := executeCLI(args, strings.NewReader(""), &stdout, &stderr)
	response := decodeResponse(t, stdout.Bytes())
	if code != 1 || response["error"].(map[string]any)["code"] != "deadline_exceeded" {
		t.Fatalf("active timeout: exit=%d, response=%#v, stderr=%q", code, response, stderr.String())
	}
	for _, file := range []string{marker, marker + ".closed"} {
		if _, err := os.Stat(file); err != nil {
			t.Fatalf("provider lifecycle %s: %v", filepath.Base(file), err)
		}
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("timeout wrote WAV: %v", err)
	}
}
