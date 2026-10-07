package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"utautts/cmd/tools/internal/sourcephone"
	"utautts/internal/audio"
)

func TestPrepareImport(t *testing.T) {
	os.MkdirAll("out", 0755)
	dir, err := os.MkdirTemp("out", "mfa-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	wav := filepath.Join(dir, "sample.wav")
	if err := audio.WriteWav(wav, &audio.PCM{SampleRate: 16000, Channels: 1, Data: make([]int16, 4800)}); err != nil {
		t.Fatal(err)
	}
	record := sourcephone.Object{"id": "TEST01", "audio_path": sourcephone.Absolute(wav), "tokens": []any{sourcephone.Object{"mora": "か", "pause": false}, sourcephone.Object{"mora": "き", "pause": false}}}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	dataset := filepath.Join(dir, "data.jsonl")
	if err := os.WriteFile(dataset, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "prepared")
	prepared, skipped, words, err := prepare([]string{dataset}, out)
	if err != nil || prepared != 1 || skipped != 0 || words != 2 {
		t.Fatalf("%d %d %d %v", prepared, skipped, words, err)
	}
	dictionary, err := os.ReadFile(filepath.Join(out, "dictionary.dict"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.ReplaceAll(string(dictionary), "\r\n", "\n") != "test01_000\tk a\ntest01_001\tc i\n" {
		t.Fatal(string(dictionary))
	}
	if _, _, _, err := prepare([]string{dataset}, out); err == nil {
		t.Fatal("overwrote existing corpus")
	}
	alignments := filepath.Join(dir, "alignments")
	os.MkdirAll(alignments, 0755)
	alignment := sourcephone.Object{"tiers": sourcephone.Object{"words": sourcephone.Object{"entries": []any{[]any{0.0, .1, "test01_000"}, []any{.1, .2, "test01_001"}}}, "phones": sourcephone.Object{"entries": []any{[]any{0.0, .1, "a"}, []any{.1, .2, "i"}}}}}
	if err := sourcephone.Write(filepath.Join(alignments, "TEST01.json"), alignment); err != nil {
		t.Fatal(err)
	}
	result := filepath.Join(dir, "aligned.jsonl")
	written, missing, err := importAlignments([]string{dataset}, alignments, result)
	if err != nil || written != 1 || missing != 0 {
		t.Fatalf("%d %d %v", written, missing, err)
	}
	raw, err := os.ReadFile(result)
	if err != nil {
		t.Fatal(err)
	}
	var aligned sourcephone.Object
	if err := json.Unmarshal(raw, &aligned); err != nil {
		t.Fatal(err)
	}
	if aligned["alignment_source"] != "mfa_japanese_note" || sourcephone.Number(sourcephone.Map(sourcephone.List(aligned["tokens"])[0])["end_ms"]) != 100 {
		t.Fatal(aligned)
	}
	if _, _, err := importAlignments([]string{dataset}, alignments, result); err == nil {
		t.Fatal("overwrote existing alignment")
	}
	if err := runWorkflow([]string{dataset}, filepath.Join(dir, "workflow"), "utautts-nonexistent-mfa-command", "japanese_mfa"); err == nil || !strings.Contains(err.Error(), "MFA alignment failed") {
		t.Fatalf("expected external MFA invocation error: %v", err)
	}
}
