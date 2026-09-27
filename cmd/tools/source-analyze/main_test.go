package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"utautts/internal/audio"
	"utautts/internal/plan"
)

func TestSourceAuditKeepsContextSeparateFromObservedPhones(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.wav")
	if err := audio.WriteWav(source, &audio.PCM{SampleRate: 16000, Channels: 1, Data: make([]int16, 1600)}); err != nil {
		t.Fatal(err)
	}
	p := plan.Plan{Language: "en", Units: []plan.Unit{
		{Position: 0, Alias: "first", Source: source},
		{Position: 2, Alias: "second", Role: "ending", Source: source, CodaPhones: []string{"d"}},
	}}
	data, _ := json.Marshal(p)
	path := filepath.Join(dir, "plan.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "audit")
	if err := run(path, out, "2"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(out, "observations.json"))
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Units    []observation `json:"units"`
		Observed []any         `json:"observed_phone_intervals"`
		Status   string        `json:"annotation_status"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Units) != 1 || report.Units[0].UnitIndex != 1 || len(report.Observed) != 0 || report.Status != "unobserved" {
		t.Fatalf("%+v", report)
	}
	if report.Units[0].Analysis.TimeOrigin != "oto-offset" {
		t.Fatal("incorrect source coordinate origin")
	}
	if err := run(path, out, "-1"); err == nil {
		t.Fatal("negative position accepted")
	}
	if err := run(path, out, "3"); err == nil {
		t.Fatal("empty selection accepted")
	}
}
