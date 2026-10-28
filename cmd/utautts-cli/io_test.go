package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yh2237/UtauWLD/runtimepath"
	"github.com/yh2237/audiodsp/wav"
)

func synthesisArgs(t *testing.T) []string {
	t.Helper()
	root := t.TempDir()
	var audio bytes.Buffer
	samples := make([]int16, 8000)
	for i := range samples {
		samples[i] = 2000
	}
	if err := wav.Encode(&audio, &wav.PCM{SampleRate: 16000, Channels: 1, Data: samples}); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{
		"a.wav":   audio.Bytes(),
		"oto.ini": []byte("a.wav=あ,0,40,0,60,30\n"),
		// doctorが探すWORLDのライブラリ。合成はテスト用の描画で行う。
		filepath.Join("runtime", runtimepath.LibraryName("utautts-world-engine")): []byte("fixture"),
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(root)
	return []string{"--voicebank", root, "--model-dir", root, "--prosody", "none", "--method", "waveform-synthesis", "--apply-pitch=false"}
}

func TestCLIPipeMatchesFileOutput(t *testing.T) {
	args := synthesisArgs(t)
	path := filepath.Join(t.TempDir(), "speech.wav")
	var stdout, stderr bytes.Buffer
	if err := runCLI(append(args, "--text", "あ", "--out", path, "--write-text", "--write-lab"), strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	baseline, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stdout.String(), "wrote ") {
		t.Fatalf("file summary = %q", stdout.String())
	}
	text, err := os.ReadFile(strings.TrimSuffix(path, ".wav") + ".txt")
	if err != nil || string(text) != "あ\n" {
		t.Fatalf("text sidecar = %q, %v", text, err)
	}
	if label, err := os.ReadFile(strings.TrimSuffix(path, ".wav") + ".lab"); err != nil || !bytes.Contains(label, []byte(" a\n")) {
		t.Fatalf("label sidecar = %q, %v", label, err)
	}
	readingPath := filepath.Join(t.TempDir(), "reading.txt")
	if err := os.WriteFile(readingPath, []byte("あ\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, input := range []struct {
		name, flag, path, stdin string
	}{
		{"text stdin", "--text-file", "-", "\ufeffあ\r\n"},
		{"reading stdin", "--reading-file", "-", "あ\n"},
		{"reading file", "--reading-file", readingPath, ""},
	} {
		t.Run(input.name, func(t *testing.T) {
			stdout.Reset()
			stderr.Reset()
			if err := runCLI(append(args, input.flag, input.path, "--out", "-"), strings.NewReader(input.stdin), &stdout, &stderr); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(stdout.Bytes(), baseline) || !strings.HasPrefix(stderr.String(), "wrote - ") {
				t.Fatalf("pipe output differs or summary is missing: stdout=%d bytes, stderr=%q", stdout.Len(), stderr.String())
			}
		})
	}
}

func TestCLIRejectsInvalidStreamInputs(t *testing.T) {
	for _, test := range []struct {
		name, stdin, message string
		args                 []string
	}{
		{"conflicting text", "", "conflicts", []string{"--text", "", "--text-file", "-"}},
		{"conflicting reading alias", "", "conflicts", []string{"--kana", "あ", "--reading-file", "-"}},
		{"two stdin consumers", "", "only one input", []string{"--text-file", "-", "--reading-file", "-"}},
		{"sidecar with stdout", "", "WAV file path", []string{"--reading", "あ", "--write-lab"}},
		{"empty stdin", " \r\n", "input is empty", []string{"--text-file", "-"}},
		{"invalid UTF-8", "\xff", "UTF-8", []string{"--text-file", "-"}},
		{"oversized stdin", strings.Repeat("a", maxUtteranceBytes+1), "limited", []string{"--text-file", "-"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			args := append([]string{"--voicebank", t.TempDir(), "--out", "-"}, test.args...)
			err := runCLI(args, strings.NewReader(test.stdin), &stdout, &stderr)
			if err == nil || !strings.Contains(err.Error(), test.message) || stdout.Len() != 0 {
				t.Fatalf("invalid input: error=%v, stdout=%q", err, stdout.String())
			}
		})
	}
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestCLIOutputFailures(t *testing.T) {
	args := append(synthesisArgs(t), "--reading", "あ", "--out", "-")
	want := errors.New("closed output pipe")
	if err := runCLI(args, strings.NewReader(""), failingWriter{want}, io.Discard); !errors.Is(err, want) {
		t.Fatalf("WAV writer failure = %v", err)
	}
	var stdout bytes.Buffer
	missingPlan := filepath.Join(t.TempDir(), "missing", "plan.json")
	if err := runCLI(append(args, "--plan-out", missingPlan), strings.NewReader(""), &stdout, io.Discard); err == nil || stdout.Len() != 0 {
		t.Fatalf("failed auxiliary output: error=%v, stdout=%d bytes", err, stdout.Len())
	}
}
