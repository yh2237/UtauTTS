package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIListsCatalogsWithoutSynthesisArguments(t *testing.T) {
	base := synthesisArgs(t)
	root := base[1]
	t.Chdir(root)
	files := map[string]string{
		"character.txt": "name=CLI fixture\n",
		"model.json": `{
			"id":"cli-test-model","display_name":"CLI test model","language":"ja",
			"version":8,"feature_version":1,"mode":"intonation_frame_tcn_accent_bounded","duration_weights":{},
			"frame_pitch":{"feature_names":["bias"],"input_weights":[[0]],"input_bias":[0],"layers":[],"output_weight":[0],"output_bias":0,"frame_ms":10,"low_cents":-100,"high_cents":100}
		}`,
		"missing/renderer.json": `{
			"manifest_version":2,"kind":"synthesis-engine","id":"cli-test-missing",
			"display_name":"Missing provider","contract":"unit-renderer","contract_version":1,"provider":"cli-test-external","provider_version":"1",
			"protocol":"utautts-provider","protocol_version":1,
			"resources":{"provider_executable":{"path":"missing-provider","required":true,"executable":true}}
		}`,
		"invalid/renderer.json": `{"manifest_version":2}`,
	}
	for name, data := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, kind := range []string{"voicebanks", "models", "renderers", "resamplers", "wavtools"} {
		t.Run(kind, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			args := []string{"--json", "--list", kind, "--voicebank", root, "--renderer-dir", root, "--model-dir", root}
			if code := executeCLI(args, strings.NewReader(""), &stdout, &stderr); code != 0 {
				t.Fatalf("exit=%d, stderr=%q", code, stderr.String())
			}
			response := decodeResponse(t, stdout.Bytes())
			catalog, ok := response["catalog"].(map[string]any)
			if response["ok"] != true || response["result"] != nil || !ok || catalog["kind"] != kind {
				t.Fatalf("catalog response = %#v", response)
			}
			items, ok := catalog["items"].([]any)
			if !ok {
				t.Fatalf("items is not an array: %#v", catalog)
			}
			switch kind {
			case "voicebanks":
				if len(items) != 1 || items[0].(map[string]any)["name"] != "CLI fixture" || items[0].(map[string]any)["path"] != root {
					t.Fatalf("voicebanks = %#v", items)
				}
			case "models":
				if len(items) != 1 || items[0].(map[string]any)["id"] != "cli-test-model" || catalog["default_id"] != "none" {
					t.Fatalf("models = %#v", catalog)
				}
			case "renderers":
				byID := make(map[string]map[string]any)
				for _, item := range items {
					value := item.(map[string]any)
					byID[value["id"].(string)] = value
				}
				if byID["cli-test-waveform"]["available"] != true || byID["cli-test-missing"]["available"] != false || len(byID["cli-test-missing"]["problems"].([]any)) == 0 {
					t.Fatalf("renderer availability = %#v", byID)
				}
				if len(catalog["problems"].([]any)) == 0 {
					t.Fatal("invalid plugin was not reported")
				}
			case "resamplers":
				if len(items) != 0 {
					t.Fatalf("unexpected resamplers = %#v", items)
				}
			case "wavtools":
				if len(items) != 1 || items[0].(map[string]any)["id"] != "builtin" {
					t.Fatalf("wavtools = %#v", items)
				}
			}
			stdout.Reset()
			if code := executeCLI(args[1:], strings.NewReader(""), &stdout, &stderr); code != 0 || stdout.Len() == 0 {
				t.Fatalf("human listing: exit=%d, stdout=%q, stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestCLICatalogConflictsAndOutputFailure(t *testing.T) {
	for _, args := range [][]string{
		{"--json", "--list", "unknown"},
		{"--json", "--list", "models", "--request", "-"},
		{"--json", "--list", "models", "--text", "あ"},
	} {
		var stdout, stderr bytes.Buffer
		code := executeCLI(args, strings.NewReader(""), &stdout, &stderr)
		response := decodeResponse(t, stdout.Bytes())
		if code != 1 || response["error"].(map[string]any)["code"] != "invalid_arguments" {
			t.Fatalf("invalid catalog command: exit=%d, response=%#v", code, response)
		}
	}
	if code := executeCLI([]string{"--list", "voicebanks", "--voicebank", t.TempDir()}, strings.NewReader(""), failingWriter{io.ErrClosedPipe}, io.Discard); code != 1 {
		t.Fatalf("catalog output failure exit=%d", code)
	}
}
