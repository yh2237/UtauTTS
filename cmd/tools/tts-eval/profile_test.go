package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"utautts/internal/render/worldline"
	"utautts/internal/tts"
)

func TestEvaluationProfileRecordsCaseAndRestoresHooks(t *testing.T) {
	oldTTS, oldWorld := tts.Trace, worldline.Trace
	defer func() { tts.Trace, worldline.Trace = oldTTS, oldWorld }()
	calls := 0
	tts.Trace = func(string) { calls++ }
	worldline.Trace = nil
	dir := t.TempDir()
	p, err := startEvaluationProfile(dir)
	if err != nil {
		t.Fatal(err)
	}
	p.SetCase("first", "renderer", 1)
	tts.Trace("select: 12.5ms")
	p.SetCase("second", "renderer", 2)
	worldline.Trace("bridge: 3.0ms")
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	tts.Trace("restored")
	if calls != 2 || worldline.Trace != nil {
		t.Fatal("trace hooks were not forwarded/restored")
	}
	data, err := os.ReadFile(filepath.Join(dir, "phase-profile.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	for i, wantID := range []string{"first", "second"} {
		var row phaseMeasurement
		if err := decoder.Decode(&row); err != nil {
			t.Fatal(err)
		}
		if row.ID != wantID || row.Repetition != i+1 || row.Renderer != "renderer" || row.ElapsedMS == nil {
			t.Fatalf("incorrect case association: %+v", row)
		}
		wantSource, wantPhase, wantMS := "tts", "select", 12.5
		if i == 1 {
			wantSource, wantPhase, wantMS = "worldline", "bridge", 3
		}
		if row.Source != wantSource || row.Phase != wantPhase || *row.ElapsedMS != wantMS {
			t.Fatalf("incorrect phase measurement: %+v", row)
		}
	}
	for _, name := range []string{"cpu.pprof", "allocs.pprof", "memory.json"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil || info.Size() == 0 {
			t.Fatalf("profile artifact %s: %v", name, err)
		}
	}
}
