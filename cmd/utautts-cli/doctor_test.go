package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIDoctorReportsPlacementWithoutRunningHelper(t *testing.T) {
	base := synthesisArgs(t)
	root := base[1]
	helper := filepath.Join(root, "not-an-executable")
	if err := os.WriteFile(helper, []byte("must not be executed"), 0600); err != nil {
		t.Fatal(err)
	}
	args := append(base[:6:6], "--doctor", "--json", "--openjtalk-engine", helper, "--openjtalk-dictionary", root)
	var stdout, stderr bytes.Buffer
	if code := executeCLI(args, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("exit=%d, stderr=%q", code, stderr.String())
	}
	response := decodeResponse(t, stdout.Bytes())
	doctor := response["doctor"].(map[string]any)
	checks := doctorChecks(t, doctor)
	if response["ok"] != true || response["result"] != nil || doctor["os"] == "" || doctor["arch"] == "" {
		t.Fatalf("doctor response = %#v", response)
	}
	for _, id := range []string{"world_engine", "model", "voicebank", "openjtalk_engine", "openjtalk_dictionary"} {
		if checks[id]["status"] != "ok" {
			t.Fatalf("check %s = %#v", id, checks[id])
		}
	}
	if checks["openjtalk_engine"]["path"] != helper {
		t.Fatalf("helper path = %#v", checks["openjtalk_engine"])
	}
	stdout.Reset()
	if code := executeCLI(append(base[:6:6], "--doctor"), strings.NewReader(""), &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "world_engine\t") {
		t.Fatalf("text diagnosis: exit=%d, stdout=%q, stderr=%q", code, stdout.String(), stderr.String())
	}
}

func doctorChecks(t *testing.T, doctor map[string]any) map[string]map[string]any {
	t.Helper()
	result := make(map[string]map[string]any)
	for _, raw := range doctor["checks"].([]any) {
		check := raw.(map[string]any)
		result[check["id"].(string)] = check
	}
	return result
}

func TestCLIDoctorKeepsChecksOnFailure(t *testing.T) {
	base := synthesisArgs(t)
	for _, test := range []struct {
		id    string
		flags []string
	}{
		{"model", []string{"--prosody", "missing-model"}},
		{"voicebank", []string{"--voicebank", filepath.Join(t.TempDir(), "missing")}},
		{"openjtalk_engine", []string{"--openjtalk-engine", filepath.Join(t.TempDir(), "missing")}},
		{"openjtalk_dictionary", []string{"--openjtalk-dictionary", filepath.Join(t.TempDir(), "missing")}},
	} {
		t.Run(test.id, func(t *testing.T) {
			args := append(base[:6:6], "--doctor", "--json")
			var stdout, stderr bytes.Buffer
			code := executeCLI(append(args, test.flags...), strings.NewReader(""), &stdout, &stderr)
			response := decodeResponse(t, stdout.Bytes())
			checks := doctorChecks(t, response["doctor"].(map[string]any))
			if code != 1 || response["ok"] != false || response["error"].(map[string]any)["code"] != "unavailable" || checks[test.id]["status"] != "error" || len(checks) < 5 {
				t.Fatalf("failed diagnosis: exit=%d, response=%#v", code, response)
			}
		})
	}
	var stdout, stderr bytes.Buffer
	args := append(base[2:6:6], "--doctor", "--json")
	if code := executeCLI(args, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("optional resources: exit=%d, stderr=%q", code, stderr.String())
	}
	checks := doctorChecks(t, decodeResponse(t, stdout.Bytes())["doctor"].(map[string]any))
	if checks["voicebank"]["status"] != "warning" {
		t.Fatalf("unselected voicebank = %#v", checks)
	}
}

func TestCLIDoctorRejectsOtherModesAndOutputFailure(t *testing.T) {
	for _, flags := range [][]string{{"--list", "models"}, {"--text", "あ"}} {
		var stdout, stderr bytes.Buffer
		code := executeCLI(append([]string{"--doctor", "--json"}, flags...), strings.NewReader(""), &stdout, &stderr)
		response := decodeResponse(t, stdout.Bytes())
		if code != 1 || response["error"].(map[string]any)["code"] != "invalid_arguments" {
			t.Fatalf("conflict: exit=%d, response=%#v", code, response)
		}
	}
	base := synthesisArgs(t)
	if code := executeCLI(append(base[:6:6], "--doctor"), strings.NewReader(""), failingWriter{io.ErrClosedPipe}, io.Discard); code != 1 {
		t.Fatalf("doctor write error: exit=%d", code)
	}
}
