package main

import (
	"bytes"
	"flag"
	"regexp"
	"strings"
	"testing"
)

func TestCLIHelpGoesToStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := executeCLI([]string{"--help"}, strings.NewReader(""), &stdout, &stderr); code != exitOK {
		t.Fatalf("help exit=%d, stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("help wrote to stderr: %q", stderr.String())
	}
	text := stdout.String()
	for _, want := range []string{
		"usage:\n", "examples:\n",
		"\ncommon:\n", "\ninput:\n", "\noutput:\n", "\nvoicebank:\n", "\nprosody:\n", "\ntiming:\n", "\ndiagnostics:\n",
		"--voicebank <string>", "--help",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("help is missing %q:\n%s", want, text)
		}
	}
	if examples := strings.Count(text, "\n  # "); examples != 5 {
		t.Fatalf("help has %d examples, want 5:\n%s", examples, text)
	}
}

func TestJSONRequested(t *testing.T) {
	flags := flag.NewFlagSet("utautts-cli", flag.ContinueOnError)
	flags.Bool("json", false, "")
	flags.Bool("apply-pitch", true, "")
	flags.String("text", "", "")
	flags.String("voicebank", "", "")
	for _, test := range []struct {
		args []string
		want bool
	}{
		{nil, false},
		{[]string{"--json"}, true},
		{[]string{"-json"}, true},
		{[]string{"--json=false"}, false},
		{[]string{"--json=false", "--json"}, true},
		{[]string{"--json", "--json=false"}, false},
		{[]string{"--text", "json"}, false},
		{[]string{"--text=json"}, false},
		{[]string{"--voicebank", "voice", "--json"}, true},
		{[]string{"--apply-pitch", "--json"}, true},
		{[]string{"--", "--json"}, false},
	} {
		if got := jsonRequested(flags, test.args); got != test.want {
			t.Fatalf("jsonRequested(%v) = %v, want %v", test.args, got, test.want)
		}
	}
}

func TestUsageListsEveryFlagOnce(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := executeCLI([]string{"--help"}, strings.NewReader(""), &stdout, &stderr); code != exitOK {
		t.Fatalf("help exit=%d", code)
	}
	seen := make(map[string]int)
	for _, match := range regexp.MustCompile(`(?m)^  (--[a-z-]+)`).FindAllStringSubmatch(stdout.String(), -1) {
		seen[match[1]]++
	}
	want := []string{
		"--json", "--version", "--timeout", "--help",
		"--text", "--reading", "--kana", "--text-file", "--reading-file", "--request",
		"--language", "--phonemizer", "--dictionary", "--method",
		"--out", "--plan-out", "--ustx-out", "--write-text", "--write-lab", "--text-encoding",
		"--voicebank", "--tone", "--color", "--alias-policy", "--reference-blend", "--openjtalk-engine", "--openjtalk-dictionary",
		"--prosody", "--model-dir", "--pitch-contours", "--pitch-case", "--apply-pitch", "--intonation-strength",
		"--manual-pitch", "--prosody-features", "--prosody-feature-case", "--prosody-pitch-only",
		"--context-duration", "--context-duration-strength", "--boundary-tone", "--boundary-tone-strength",
		"--stretch-adapt", "--stretch-adapt-strength", "--pause-context", "--pause-context-strength",
		"--microprosody", "--timing-warp",
		"--mora-ms", "--pause-ms", "--leading-preutterance-ms", "--release-ms",
		"--mora-durations", "--phone-weights", "--phone-lengths",
		"--cvvc-timing", "--cvvc-transition-gain", "--cvvc-pre-boundary-fade",
		"--list", "--doctor",
	}
	for _, name := range want {
		if seen[name] != 1 {
			t.Fatalf("%s appears %d times in --help", name, seen[name])
		}
	}
	if len(seen) != len(want) {
		t.Fatalf("--help lists %d flags, want %d", len(seen), len(want))
	}
}
